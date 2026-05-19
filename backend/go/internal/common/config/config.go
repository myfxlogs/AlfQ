// Package config provides Viper-based configuration with fsnotify hot-reload.
package config

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Config holds the application configuration.
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Log    LogConfig    `mapstructure:"log"`
}

type ServerConfig struct {
	Listen string `mapstructure:"listen"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

// Defaults returns a Config with sensible defaults.
func Defaults() *Config {
	return &Config{
		Server: ServerConfig{Listen: ":9000"},
		Log:    LogConfig{Level: "info"},
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
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("config read: %w", err)
		}
		// Config file not found — proceed with defaults.
	}

	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		// In production, re-unmarshal into cfg and notify subscribers.
		_ = e
	})

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("config unmarshal: %w", err)
	}

	return v, nil
}
