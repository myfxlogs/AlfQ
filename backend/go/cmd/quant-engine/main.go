// quant-engine merges factor-svc and strategy-svc into a single process.
package main

import (
	"fmt"
	"os"

	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/quantengine"
)

func main() {
	if err := bootstrap.Run("quant-engine", register,
		bootstrap.WithoutPG(),
		bootstrap.WithoutRedis(),
	); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
		os.Exit(1)
	}
}

func register(adapter *bootstrap.ServeMuxAdapter, d *bootstrap.Deps) error {
	return quantengine.RunQuantEngine(adapter.Mux, d)
}
