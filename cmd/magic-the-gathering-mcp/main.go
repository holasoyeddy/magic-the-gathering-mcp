package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/cardcache"
	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/scryfall"
	cardserver "github.com/holasoyeddy/magic-the-gathering-mcp/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	logger := log.New(os.Stderr, "magic-the-gathering-mcp: ", log.LstdFlags)
	live := scryfall.NewAdapter("https://api.scryfall.com", http.DefaultClient)
	adapter, err := cardcache.New(live, cardcache.Options{Diagnostic: logger.Printf})
	if err != nil {
		logger.Printf("initialize cache: %v", err)
		os.Exit(1)
	}
	if err := cardserver.New(adapter).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		logger.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}
