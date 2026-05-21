// Package config provides Viper-based configuration with fsnotify hot-reload.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Config holds the application configuration.
type Config struct {
	Server     ServerConfig  `mapstructure:"server"`
	Log        LogConfig     `mapstructure:"log"`
	MT4Gateway GatewayConfig `mapstructure:"mt4_gateway"`
	MT5Gateway GatewayConfig `mapstructure:"mt5_gateway"`
}

type ServerConfig struct {
	Listen string `mapstructure:"listen"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

type GatewayConfig struct {
	Addr    string        `mapstructure:"addr"`
	UseTLS  bool          `mapstructure:"use_tls"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// Defaults returns a Config with sensible defaults.
func Defaults() *Config {
	return &Config{
		Server: ServerConfig{Listen: ":9000"},
		Log:    LogConfig{Level: "info"},
		MT4Gateway: GatewayConfig{
			Addr:    "mt4grpc3.mtapi.io:443",
			UseTLS:  true,
			Timeout: 30 * time.Second,
		},
		MT5Gateway: GatewayConfig{
			Addr:    "mt5grpc3.mtapi.io:443",
			UseTLS:  true,
			Timeout: 30 * time.Second,
		},
	}
}

// Load reads config from the given YAML path. Falls back to defaults if the file
// is missing. Enables fsnotify-based hot-reload via viper.
func Load(path string, cfg *Config) (*viper.Viper, error) {
	if cfg == nil {
		cfg = Defaults()
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.SetDefault("server.listen", cfg.Server.Listen)
	v.SetDefault("log.level", cfg.Log.Level)

	if err := v.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no such file") {
			return nil, fmt.Errorf("config read: %w", err)
		}
	}

	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		_ = e
	})

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("config unmarshal: %w", err)
	}

	return v, nil
}
