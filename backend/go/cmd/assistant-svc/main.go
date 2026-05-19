// assistant-svc is the AI Strategy Assistant service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/alfq/backend/go/internal/assistantsvc"
	"github.com/alfq/backend/go/internal/common/config"
	"github.com/alfq/backend/go/internal/common/logger"
)

func main() {
	cfg := config.Defaults()
	log, err := logger.New(cfg.Log.Level)
	if err != nil {
		slog.Error("logger init failed")
		os.Exit(1)
	}
	defer log.Sync()

	registry := assistantsvc.NewRegistry()

	// M4.5: Load knowledge base
	kb := assistantsvc.NewKnowledgeBase()
	if err := kb.Load("docs"); err != nil {
		log.Warn("kb load failed", zap.Error(err))
	} else {
		registry.SetKB(kb)
	}

	// M6.5: Cloud LLM provider abstraction (ADR 0009)
	// API keys from environment variables (fallback to empty for local dev)
	openaiKey := os.Getenv("OPENAI_API_KEY")
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")

	router := assistantsvc.NewRouter()
	if openaiKey != "" {
		router.Register(
			&assistantsvc.Provider{Name: "openai", Endpoint: "https://api.openai.com", Model: "gpt-4o", Priority: 1, Timeout: 30 * time.Second},
			assistantsvc.NewHTTPClient("openai", "https://api.openai.com", "gpt-4o", openaiKey),
		)
	}
	if anthropicKey != "" {
		router.Register(
			&assistantsvc.Provider{Name: "anthropic", Endpoint: "https://api.anthropic.com", Model: "claude-sonnet-4-20250514", Priority: 2, Timeout: 30 * time.Second},
			assistantsvc.NewHTTPClient("anthropic", "https://api.anthropic.com", "claude-sonnet-4-20250514", anthropicKey),
		)
	}

	tools := registry.List()
	log.Info("assistant-svc starting",
		zap.Int("tools", len(tools)),
	)

	// Chat endpoint: accept user message and route to LLM.
	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		msg := r.PostFormValue("message")
		if msg == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Route to the highest-priority provider.
		resp, _ := router.Chat(r.Context(), "You are a trading assistant.", msg)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"response": %q}`, resp)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})
	mux.Handle("/metrics", promhttp.Handler())
	// Also mount legacy endpoints on the same mux.
	mux.HandleFunc("/tools", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"tools": %d}`, len(tools))
	})
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
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
		fmt.Fprintf(w, `{"response": %q}`, resp)
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	server := &http.Server{Addr: ":9003", Handler: mux}
	go func() {
		log.Info("assistant-svc starting", zap.String("addr", ":9003"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")
	shutdownCtx, sdCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sdCancel()
	server.Shutdown(shutdownCtx)
	log.Info("assistant-svc stopped")
}
