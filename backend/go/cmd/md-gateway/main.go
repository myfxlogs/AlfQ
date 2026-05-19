// md-gateway is the ALFQ market data gateway.
package main

import (
	"fmt"
	"os"

	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/mdgateway"
	"github.com/spf13/viper"
)

func main() {
	if err := bootstrap.Run("md-gateway", register, bootstrap.WithoutPG(), bootstrap.WithoutRedis()); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
		os.Exit(1)
	}
}

func register(adapter *bootstrap.ServeMuxAdapter, d *bootstrap.Deps) error {
	v := viper.New()
	v.SetConfigName("md-gateway")
	v.SetConfigType("yaml")
	v.AddConfigPath("configs")
	v.SetDefault("nats.url", os.Getenv("NATS_URL"))
	v.SetDefault("redis.addr", os.Getenv("REDIS_ADDR"))
	if v.GetString("nats.url") == "" { v.Set("nats.url", "nats://localhost:4222") }
	if v.GetString("redis.addr") == "" { v.Set("redis.addr", "localhost:6379") }
	_ = v.ReadInConfig()

	var accounts []mdgateway.AccountEntry
	_ = v.UnmarshalKey("accounts", &accounts)

	return mdgateway.RunGateway(adapter.Mux, d, mdgateway.Config{
		Log: mdgateway.LogConfig{Level: "info"}, Accounts: accounts,
	}, v.GetString("nats.url"), v.GetString("redis.addr"))
}
