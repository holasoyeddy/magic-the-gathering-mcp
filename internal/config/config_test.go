package config_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/config"
)

func TestLoadUsesDocumentedDefaults(t *testing.T) {
	t.Setenv("MTG_MCP_CACHE_DIR", "")
	t.Setenv("MTG_MCP_CACHE_MAX_BYTES", "")
	t.Setenv("MTG_MCP_CACHE_RETENTION", "")
	t.Setenv("MTG_MCP_LOG_LEVEL", "")

	got, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got.CacheDirectory) != "magic-the-gathering-mcp" {
		t.Fatalf("cache directory = %q", got.CacheDirectory)
	}
	if got.CacheMaxBytes != 256<<20 || got.CacheRetention != 30*24*time.Hour || got.LogLevel != "info" {
		t.Fatalf("defaults = %#v", got)
	}
}

func TestLoadAcceptsEnvironmentOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MTG_MCP_CACHE_DIR", dir)
	t.Setenv("MTG_MCP_CACHE_MAX_BYTES", "1048576")
	t.Setenv("MTG_MCP_CACHE_RETENTION", "72h")
	t.Setenv("MTG_MCP_LOG_LEVEL", "debug")

	got, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.CacheDirectory != dir || got.CacheMaxBytes != 1048576 || got.CacheRetention != 72*time.Hour || got.LogLevel != "debug" {
		t.Fatalf("overrides = %#v", got)
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	tests := []struct{ key, value string }{
		{"MTG_MCP_CACHE_MAX_BYTES", "0"},
		{"MTG_MCP_CACHE_MAX_BYTES", "lots"},
		{"MTG_MCP_CACHE_RETENTION", "0s"},
		{"MTG_MCP_CACHE_RETENTION", "1h"},
		{"MTG_MCP_CACHE_RETENTION", "forever"},
		{"MTG_MCP_CACHE_RETENTION", "721h"},
		{"MTG_MCP_LOG_LEVEL", "verbose"},
	}
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			t.Setenv("MTG_MCP_CACHE_DIR", t.TempDir())
			t.Setenv("MTG_MCP_CACHE_MAX_BYTES", "")
			t.Setenv("MTG_MCP_CACHE_RETENTION", "")
			t.Setenv("MTG_MCP_LOG_LEVEL", "")
			t.Setenv(tt.key, tt.value)
			if _, err := config.Load(); err == nil {
				t.Fatal("Load succeeded, want startup error")
			}
		})
	}
}
