package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/cardcache"
	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/config"
	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/scryfall"
	cardserver "github.com/holasoyeddy/magic-the-gathering-mcp/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	logger := log.New(os.Stderr, "magic-the-gathering-mcp: ", log.LstdFlags)
	cfg, err := config.Load()
	if err != nil {
		logger.Printf("invalid configuration: %v", err)
		os.Exit(2)
	}
	live := scryfall.NewAdapter("https://api.scryfall.com", http.DefaultClient)
	diagnostic := func(string, ...any) {}
	if cfg.LogLevel != "error" {
		diagnostic = logger.Printf
	}
	adapter, err := cardcache.New(live, cardcache.Options{Directory: cfg.CacheDirectory, MaxBytes: cfg.CacheMaxBytes, RetainFor: cfg.CacheRetention, Diagnostic: diagnostic})
	if err != nil {
		logger.Printf("initialize cache: %v", err)
		os.Exit(1)
	}
	if err := cardserver.New(adapter).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		logger.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}
