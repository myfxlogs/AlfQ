package assistantsvc

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/alfq/backend/go/internal/common/bootstrap"
	"go.uber.org/zap"
)

// RunAssistant wires the AI assistant service and registers its routes on mux.
func RunAssistant(mux *http.ServeMux, d *bootstrap.Deps) error {
	registry := NewRegistry()

	kb := NewKnowledgeBase()
	if err := kb.Load("docs"); err != nil {
		d.Log.Warn("kb load failed", zap.Error(err))
	} else {
		registry.SetKB(kb)
	}

	openaiKey := os.Getenv("OPENAI_API_KEY")
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")

	router := NewRouter()
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

	d.Log.Info("assistant-svc starting", zap.Int("tools", len(registry.List())))

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})
	mux.HandleFunc("/chat", chatHandler(router))
	mux.HandleFunc("/tools", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"tools": %d}`, len(registry.List())) //nolint:errcheck
	})

	return nil
}

func chatHandler(router *Router) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		msg := r.PostFormValue("message")
		if msg == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		resp, _ := router.Chat(r.Context(), "You are a trading assistant.", msg)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"response": %q}`, resp) //nolint:errcheck
	}
}
