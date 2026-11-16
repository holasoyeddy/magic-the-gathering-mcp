package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"regexp"
	"strings"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func New(adapter card.Adapter) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "magic-the-gathering-mcp", Version: "v0.1.0"},
		&mcp.ServerOptions{Capabilities: &mcp.ServerCapabilities{}},
	)
	server.AddTool(&mcp.Tool{
		Name:        "get_oracle_card",
		Description: "Resolve an Oracle card by exact name or Oracle ID.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"name":      map[string]any{"type": "string", "description": "Canonical card name matched exactly, case-insensitively."},
				"oracle_id": map[string]any{"type": "string", "format": "uuid", "description": "Oracle identity UUID."},
			},
		},
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input struct {
			Name     string `json:"name"`
			OracleID string `json:"oracle_id"`
		}
		if err := json.Unmarshal(request.Params.Arguments, &input); err != nil {
			return toolError("invalid_identifier", "provide exactly one valid selector"), nil
		}
		input.Name = strings.TrimSpace(input.Name)
		input.OracleID = strings.TrimSpace(input.OracleID)
		if (input.Name == "") == (input.OracleID == "") || (input.OracleID != "" && !uuidPattern.MatchString(input.OracleID)) {
			return toolError("invalid_identifier", "provide exactly one valid name or Oracle ID"), nil
		}
		result, err := adapter.GetOracleCard(ctx, card.OracleCardSelector{Name: input.Name, OracleID: input.OracleID})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			if errors.Is(err, card.ErrNotFound) {
				return toolError("not_found", "no Oracle card matched the exact selector"), nil
			}
			for expected, problem := range map[error][2]string{
				card.ErrInvalidQuery:        {"invalid_query", "Scryfall rejected the query"},
				card.ErrInvalidIdentifier:   {"invalid_identifier", "the card identifier is invalid"},
				card.ErrRateLimited:         {"rate_limited", "the request could not be admitted within its time limit"},
				card.ErrUpstreamUnavailable: {"upstream_unavailable", "Scryfall is temporarily unavailable"},
			} {
				if errors.Is(err, expected) {
					return toolError(problem[0], problem[1]), nil
				}
			}
			log.Printf("get_oracle_card failed: %v", err)
			return toolError("internal_error", "an internal error occurred"), nil
		}
		result.FetchedAt = result.FetchedAt.UTC()
		return &mcp.CallToolResult{StructuredContent: result}, nil
	})
	return server
}

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func toolError(code, message string) *mcp.CallToolResult {
	b, _ := json.Marshal(map[string]string{"code": code, "message": message})
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}
