package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// CacheDirectoryEnvironment names the cache-directory setting.
	CacheDirectoryEnvironment = "MTG_MCP_CACHE_DIR"
	// CacheMaxBytesEnvironment names the positive cache-capacity setting in bytes.
	CacheMaxBytesEnvironment = "MTG_MCP_CACHE_MAX_BYTES"
	// CacheRetentionEnvironment names the stale-retention duration setting.
	CacheRetentionEnvironment = "MTG_MCP_CACHE_RETENTION"
	// LogLevelEnvironment names the debug, info, or error logging setting.
	LogLevelEnvironment = "MTG_MCP_LOG_LEVEL"
)

// Config contains validated process configuration for card-fact service startup.
type Config struct {
	CacheDirectory string
	CacheMaxBytes  int64
	CacheRetention time.Duration
	LogLevel       string
}

// Load reads process configuration from the environment, applies documented
// defaults, and rejects invalid capacity, retention, and log-level values.
func Load() (Config, error) {
	dir := strings.TrimSpace(os.Getenv(CacheDirectoryEnvironment))
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return Config{}, fmt.Errorf("resolve user cache directory: %w", err)
		}
		dir = filepath.Join(base, "magic-the-gathering-mcp")
	}
	cfg := Config{CacheDirectory: dir, CacheMaxBytes: 256 << 20, CacheRetention: 30 * 24 * time.Hour, LogLevel: "info"}
	if raw := strings.TrimSpace(os.Getenv(CacheMaxBytesEnvironment)); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 1 {
			return Config{}, fmt.Errorf("%s must be a positive integer number of bytes", CacheMaxBytesEnvironment)
		}
		cfg.CacheMaxBytes = value
	}
	if raw := strings.TrimSpace(os.Getenv(CacheRetentionEnvironment)); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value < 24*time.Hour || value > 30*24*time.Hour {
			return Config{}, fmt.Errorf("%s must be a Go duration from 24h through 720h", CacheRetentionEnvironment)
		}
		cfg.CacheRetention = value
	}
	if raw := strings.ToLower(strings.TrimSpace(os.Getenv(LogLevelEnvironment))); raw != "" {
		switch raw {
		case "debug", "info", "error":
			cfg.LogLevel = raw
		default:
			return Config{}, fmt.Errorf("%s must be debug, info, or error", LogLevelEnvironment)
		}
	}
	return cfg, nil
}
