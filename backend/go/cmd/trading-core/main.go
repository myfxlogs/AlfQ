// trading-core merges admin-api, oms, and risk-svc into a single process.
package main

import (
	"fmt"
	"os"

	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/tradingcore"
)

func main() {
	if err := bootstrap.Run("trading-core", register); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
		os.Exit(1)
	}
}

func register(adapter *bootstrap.ServeMuxAdapter, d *bootstrap.Deps) error {
	mux := adapter.Mux
	_ = bootstrap.CORSMiddleware(mux)
	shutdown, err := tradingcore.RunTradingCore(mux, d)
	if err != nil {
		return err
	}
	adapter.OnShutdown = append(adapter.OnShutdown, shutdown)
	return nil
}
