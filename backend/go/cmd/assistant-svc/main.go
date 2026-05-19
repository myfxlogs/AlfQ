// assistant-svc is the AI Strategy Assistant service.
package main

import (
	"fmt"
	"os"

	"github.com/alfq/backend/go/internal/assistantsvc"
	"github.com/alfq/backend/go/internal/common/bootstrap"
)

func main() {
	if err := bootstrap.Run("assistant-svc", register,
		bootstrap.WithoutPG(),
		bootstrap.WithoutRedis(),
		bootstrap.WithoutNATS(),
		bootstrap.WithoutCH(),
	); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
		os.Exit(1)
	}
}

func register(adapter *bootstrap.ServeMuxAdapter, d *bootstrap.Deps) error {
	return assistantsvc.RunAssistant(adapter.Mux, d)
}
