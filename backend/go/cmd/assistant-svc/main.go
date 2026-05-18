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

	tools := registry.List()
	log.Info("assistant-svc starting",
		zap.Int("tools", len(tools)),
	)

	// Tool list endpoint
	http.HandleFunc("/tools", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"tools": %d}`, len(tools))
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	server := &http.Server{Addr: ":9006"}
	go func() {
		log.Info("assistant-svc starting", zap.String("addr", ":9006"))
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

	_ = fmt.Sprintf
}
