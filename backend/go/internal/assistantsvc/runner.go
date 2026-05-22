// Package assistantsvc — AI Strategy Assistant service runner (R10 upgrade).
//
// R10 adds:
//   - Multi-tenant API key isolation (reads keys from user_api_keys table)
//   - Usage tracking per user (writes to ai_usage_logs)
//   - Monthly budget enforcement (quota_limit_cents)
//   - pgvector-based RAG knowledge retrieval
package assistantsvc

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alfq/backend/go/internal/common/auth"
	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/common/crypto"
	"github.com/alfq/backend/go/internal/common/vault"
	"go.uber.org/zap"
)

// RunAssistant wires the AI assistant service with PG, JWT auth, and RAG.
func RunAssistant(mux *http.ServeMux, d *bootstrap.Deps) error {
	log := d.Log

	// ── AES encryption for reading user API keys ──
	encKey := getEncryptionKey(d)
	aesCipher, err := crypto.NewAESCipher(encKey)
	if err != nil {
		log.Warn("aes cipher init failed, key decryption disabled", zap.Error(err))
	}

	// ── Registry & Knowledge Base ──
	registry := NewRegistry()

	kb := NewKnowledgeBase()
	if err := kb.Load("docs"); err != nil {
		log.Warn("kb load failed", zap.Error(err))
	} else {
		registry.SetKB(kb)
	}

	// ── Router (without static keys — keys come from DB per user) ──
	router := NewRouter()

	// RC06: Try Vault for startup secrets (RAG embeddings), fallback to env.
	vaultSecrets := loadVaultSecrets(d)
	openaiKey := vaultSecret(vaultSecrets, "OPENAI_API_KEY")
	anthropicKey := vaultSecret(vaultSecrets, "ANTHROPIC_API_KEY")

	if openaiKey != "" {
		router.Register(
			&Provider{Name: "openai", Endpoint: "https://api.openai.com", Model: "gpt-4o", Priority: 1, Timeout: 30 * time.Second},
			NewHTTPClient("openai", "https://api.openai.com", "gpt-4o", openaiKey),
		)
	}
	if anthropicKey != "" {
		router.Register(
			&Provider{Name: "anthropic", Endpoint: "https://api.anthropic.com", Model: "claude-sonnet-4-20250514", Priority: 2, Timeout: 30 * time.Second},
			NewHTTPClient("anthropic", "https://api.anthropic.com", "claude-sonnet-4-20250514", anthropicKey),
		)
	}

	// ── RAG: Enable pgvector and router for knowledge base ──
	if d.PG != nil && len(router.List()) > 0 {
		kb.WithPG(d.PG).WithRouter(router)
		log.Info("rag: pgvector enabled")
		// Index embeddings in background
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := kb.IndexEmbeddings(ctx); err != nil {
				log.Warn("rag: embedding index failed (non-fatal)", zap.Error(err))
			} else {
				log.Info("rag: embeddings indexed")
			}
		}()
	}

	// ── JWT Auth Middleware ──
	authMW := createAuthMiddleware(d)

	log.Info("assistant-svc starting",
		zap.Int("providers", len(router.List())),
		zap.String("kb_status", kb.Status()),
	)

	// ── Routes ──
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	// Chat endpoint (R10: with JWT auth, usage tracking, budget enforcement, RAG)
	chatH := chatHandlerV2(router, kb, d, aesCipher)
	mux.HandleFunc("/chat", authMW(chatH).ServeHTTP)

	mux.HandleFunc("/tools", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"tools": %d}`, len(registry.List())) //nolint:errcheck
	})

	return nil
}

// ── JWT Auth Middleware for assistant-svc ──

func createAuthMiddleware(d *bootstrap.Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearer(r)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Load JWT public keys from system_settings (same as trading-core)
			keys := loadJWTKeys(d)
			claims, err := auth.Verify(token, keys)
			if err != nil || claims.IsExpired() {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			ctx = auth.WithTenant(ctx, claims.TenantID)
			ctx = auth.WithUser(ctx, claims.Sub)
			ctx = auth.WithRoles(ctx, claims.Roles)
			ctx = auth.WithEmail(ctx, claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func loadJWTKeys(d *bootstrap.Deps) map[string]auth.Ed25519PublicKey {
	if d.PG == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var privB64 string
	err := d.PG.QueryRow(ctx, `SELECT value FROM system_settings WHERE key='jwt_signing_key'`).Scan(&privB64)
	if err != nil || privB64 == "" {
		return nil
	}
	kp, err := auth.LoadKeyPair("persisted", privB64)
	if err != nil {
		return nil
	}
	return map[string]auth.Ed25519PublicKey{kp.Kid: kp.PublicKey}
}

func extractBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

// ── Encryption Key ──

func getEncryptionKey(d *bootstrap.Deps) []byte {
	// 1. Try env var
	if key := os.Getenv("ALFQ_ENC_KEY"); len(key) >= 32 {
		return []byte(key[:32])
	}
	// 2. Try system_settings
	if d.PG != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var keyB64 string
		if err := d.PG.QueryRow(ctx, `SELECT value FROM system_settings WHERE key='alfq_enc_key'`).Scan(&keyB64); err == nil && keyB64 != "" {
			// Derive 32-byte key from the stored value
			h := sha256.Sum256([]byte(keyB64))
			return h[:]
		}
	}
	// 3. Fallback: derive from default (development only!)
	h := sha256.Sum256([]byte("alfq-dev-encryption-key-change-in-production"))
	return h[:]
}

// ── Chat Handler V2 (R10) ──

func chatHandlerV2(router *Router, kb *KnowledgeBase, d *bootstrap.Deps, aesCipher *crypto.AESCipher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONResp(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		// Parse request
		var req struct {
			Message  string `json:"message"`
			Provider string `json:"provider"` // provider name, default "openai"
			RAG      bool   `json:"rag"`      // enable RAG knowledge retrieval
			Endpoint string `json:"endpoint"` // custom API endpoint (overrides default)
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONResp(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if req.Message == "" {
			writeJSONResp(w, http.StatusBadRequest, map[string]string{"error": "message required"})
			return
		}

		userID := auth.UserFromContext(r.Context())
		if userID == "" {
			writeJSONResp(w, http.StatusUnauthorized, map[string]string{"error": "请先登录"})
			return
		}

		// ── R10: Read API key from user_api_keys table ──
		provider := req.Provider
		if provider == "" {
			provider = "openai" // default
		}

		apiKey, model, quotaLimit, err := loadUserAPIKey(r.Context(), d, aesCipher, userID, provider)
		if err != nil {
			writeJSONResp(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		// ── R10: Budget check ──
		monthCost, err := getMonthCost(r.Context(), d, userID)
		if err == nil && quotaLimit > 0 && monthCost >= quotaLimit {
			writeJSONResp(w, http.StatusTooManyRequests, map[string]string{
				"error": fmt.Sprintf("月度预算已用尽 (本月已用 $%.2f / $%.2f)", float64(monthCost)/100, float64(quotaLimit)/100),
			})
			return
		}

		// ── RAG: Retrieve relevant docs ──
		systemPrompt := "你是 ALFQ 量化交易策略助手。用简体中文回答。"
		if req.RAG && kb != nil {
			docs := kb.Search(r.Context(), req.Message)
			if len(docs) > 0 {
				systemPrompt += "\n\n参考以下文档回答问题，如果不确定，直接说不知道：\n"
				for _, doc := range docs {
					systemPrompt += fmt.Sprintf("\n--- %s (%s) ---\n%s\n", doc.Title, doc.Path, doc.Content)
				}
			}
		}

		// ── Build per-request router with user's key ──
		userRouter := NewRouter()
		endpoint := defaultEndpoint(provider)
		if req.Endpoint != "" {
			endpoint = req.Endpoint // custom endpoint overrides default
		}
		if model == "" {
			model = defaultModel(provider)
		}
		userRouter.Register(
			&Provider{Name: provider, Endpoint: endpoint, Model: model, Priority: 1, Timeout: 30 * time.Second},
			NewHTTPClient(provider, endpoint, model, apiKey),
		)

		// ── Chat ──
		result, err := userRouter.Chat(r.Context(), systemPrompt, req.Message)
		if err != nil {
			writeJSONResp(w, http.StatusInternalServerError, map[string]string{"error": "AI 请求失败: " + err.Error()})
			return
		}

		// ── R10: Log usage ──
		costCents := estimateCost(provider, model, result.TokensIn, result.TokensOut)
		logUsage(r.Context(), d, userID, provider, model, result.TokensIn, result.TokensOut, costCents)

		writeJSONResp(w, http.StatusOK, map[string]any{
			"response":   result.Content,
			"tokens_in":  result.TokensIn,
			"tokens_out": result.TokensOut,
			"model":      result.Model,
			"cost_cents": costCents,
		})
	}
}

// ── DB helpers ──

func loadUserAPIKey(ctx context.Context, d *bootstrap.Deps, aesCipher *crypto.AESCipher, userID, provider string) (apiKey, model string, quotaLimit int, err error) {
	if d.PG == nil {
		// RC06: Fallback chain: Vault → env
		vaultSecrets := loadVaultSecrets(d)
		envKey := "OPENAI_API_KEY"
		if provider != "openai" {
			envKey = "ANTHROPIC_API_KEY"
		}
		apiKey = vaultSecret(vaultSecrets, envKey)
		if apiKey == "" {
			return "", "", 0, fmt.Errorf("未设置 %s API Key，请在设置页面配置", provider)
		}
		return apiKey, "", 0, nil
	}

	if aesCipher == nil {
		return "", "", 0, fmt.Errorf("加密未配置，无法读取 API Key")
	}

	var keyCipher string
	err = d.PG.QueryRow(ctx,
		`SELECT key_cipher, COALESCE(model,''), COALESCE(quota_limit_cents,500)
		 FROM user_api_keys WHERE user_id=$1 AND provider=$2`,
		userID, provider,
	).Scan(&keyCipher, &model, &quotaLimit)
	if err != nil {
		return "", "", 0, fmt.Errorf("未设置 %s API Key，请在设置页面配置", provider)
	}

	apiKey, err = aesCipher.Decrypt(keyCipher)
	if err != nil {
		return "", "", 0, fmt.Errorf("解密 %s API Key 失败", provider)
	}

	return apiKey, model, quotaLimit, nil
}

func getMonthCost(ctx context.Context, d *bootstrap.Deps, userID string) (int, error) {
	if d.PG == nil {
		return 0, nil
	}
	var cost int
	err := d.PG.QueryRow(ctx,
		`SELECT COALESCE(SUM(cost_cents), 0) FROM ai_usage_logs
		 WHERE user_id=$1 AND date_trunc('month', created_at) = date_trunc('month', CURRENT_DATE)`,
		userID,
	).Scan(&cost)
	return cost, err
}

func logUsage(ctx context.Context, d *bootstrap.Deps, userID, provider, model string, tokensIn, tokensOut, costCents int) {
	if d.PG == nil {
		return
	}
	_, err := d.PG.Exec(ctx,
		`INSERT INTO ai_usage_logs (user_id, provider, model, tokens_in, tokens_out, cost_cents)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, provider, model, tokensIn, tokensOut, costCents,
	)
	if err != nil {
		d.Log.Warn("ai usage log failed", zap.Error(err))
	}
}

// ── Cost estimation ──

func estimateCost(provider, model string, tokensIn, tokensOut int) int {
	// Approximate pricing in cents per 1K tokens
	// OpenAI: gpt-4o ~$2.50/1M input, $10/1M output
	// Anthropic: claude-sonnet-4 ~$3/1M input, $15/1M output
	inputRate := 0.25  // cents per 1K input tokens (default)
	outputRate := 1.00 // cents per 1K output tokens (default)

	switch {
	case provider == "openai" && strings.Contains(model, "gpt-4o"):
		inputRate = 0.25
		outputRate = 1.00
	case provider == "openai" && strings.Contains(model, "gpt-4"):
		inputRate = 3.00
		outputRate = 6.00
	case provider == "openai" && strings.Contains(model, "gpt-3.5"):
		inputRate = 0.05
		outputRate = 0.15
	case provider == "anthropic":
		inputRate = 0.30
		outputRate = 1.50
	}

	cost := float64(tokensIn)*inputRate/1000 + float64(tokensOut)*outputRate/1000
	return int(cost + 0.5) // round to nearest cent
}

// ── JSON response helper ──

func writeJSONResp(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ── Vault helpers (RC06) ──

// loadVaultSecrets tries to load secrets from Vault. Returns nil on failure.
func loadVaultSecrets(d *bootstrap.Deps) map[string]string {
	vc, err := vault.New()
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	secrets, err := vc.LoadSecrets(ctx, "secret")
	if err != nil {
		d.Log.Warn("vault load failed, falling back to env")
		return nil
	}
	d.Log.Info("secrets loaded from vault")
	return secrets
}

// defaultEndpoint returns the API base URL for a provider.
func defaultEndpoint(provider string) string {
	switch provider {
	case "openai":
		return "https://api.openai.com"
	case "anthropic":
		return "https://api.anthropic.com"
	case "deepseek":
		return "https://api.deepseek.com"
	case "groq":
		return "https://api.groq.com/openai"
	case "ollama":
		return "http://localhost:11434/v1"
	default:
		return "https://api.openai.com" // custom: caller provides endpoint
	}
}

// defaultModel returns the default model for a provider.
func defaultModel(provider string) string {
	switch provider {
	case "openai":
		return "gpt-4o"
	case "anthropic":
		return "claude-sonnet-4-20250514"
	case "deepseek":
		return "deepseek-chat"
	case "groq":
		return "llama-3.3-70b"
	case "ollama":
		return "llama3"
	default:
		return ""
	}
}

// vaultSecret returns a secret value from Vault map, or falls back to env var.
func vaultSecret(vaultSecrets map[string]string, key string) string {
	if vaultSecrets != nil {
		if v, ok := vaultSecrets[key]; ok && v != "" {
			return v
		}
	}
	return os.Getenv(key)
}
