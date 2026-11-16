package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"regexp"
	"sort"
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
			return exactLookupError("get_oracle_card", "no Oracle card matched the exact selector", err)
		}
		result.FetchedAt = result.FetchedAt.UTC()
		return &mcp.CallToolResult{StructuredContent: result}, nil
	})
	server.AddTool(&mcp.Tool{
		Name:        "get_printing",
		Description: "Resolve one exact printing by Scryfall ID or set code and collector number.",
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{
				"scryfall_id":      map[string]any{"type": "string", "format": "uuid", "description": "Scryfall printing UUID."},
				"set_code":         map[string]any{"type": "string", "description": "Case-insensitive set code."},
				"collector_number": map[string]any{"type": "string", "description": "Collector number preserved as text."},
			},
		},
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input struct {
			ScryfallID      string `json:"scryfall_id"`
			SetCode         string `json:"set_code"`
			CollectorNumber string `json:"collector_number"`
		}
		if err := json.Unmarshal(request.Params.Arguments, &input); err != nil {
			return toolError("invalid_identifier", "provide exactly one complete printing selector"), nil
		}
		input.ScryfallID = strings.TrimSpace(input.ScryfallID)
		input.SetCode = strings.ToLower(strings.TrimSpace(input.SetCode))
		input.CollectorNumber = strings.TrimSpace(input.CollectorNumber)
		byID := input.ScryfallID != "" && uuidPattern.MatchString(input.ScryfallID) && input.SetCode == "" && input.CollectorNumber == ""
		byCatalog := input.ScryfallID == "" && validSetCode.MatchString(input.SetCode) && input.CollectorNumber != ""
		if !byID && !byCatalog {
			return toolError("invalid_identifier", "provide a valid Scryfall ID or both set code and collector number"), nil
		}
		result, err := adapter.GetPrinting(ctx, card.PrintingSelector{ScryfallID: input.ScryfallID, SetCode: input.SetCode, CollectorNumber: input.CollectorNumber})
		if err != nil {
			return exactLookupError("get_printing", "no printing matched the exact selector", err)
		}
		result.FetchedAt = result.FetchedAt.UTC()
		return &mcp.CallToolResult{StructuredContent: result}, nil
	})
	server.AddTool(&mcp.Tool{
		Name:        "get_card_legality",
		Description: "Resolve one Oracle card and return its authoritative format-legality statuses.",
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
		legalityAdapter, ok := adapter.(card.LegalityAdapter)
		if !ok {
			return toolError("internal_error", "an internal error occurred"), nil
		}
		result, err := legalityAdapter.GetCardLegality(ctx, card.OracleCardSelector{Name: input.Name, OracleID: input.OracleID})
		if err != nil {
			return exactLookupError("get_card_legality", "no Oracle card matched the exact selector", err)
		}
		result.FetchedAt = result.FetchedAt.UTC()
		return &mcp.CallToolResult{StructuredContent: result}, nil
	})
	server.AddTool(&mcp.Tool{
		Name:        "get_rulings",
		Description: "Resolve official Oracle-card rulings by exact name or Oracle ID.",
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
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
		result, err := adapter.GetRulings(ctx, card.OracleCardSelector{Name: input.Name, OracleID: input.OracleID})
		if err != nil {
			return exactLookupError("get_rulings", "no Oracle card matched the exact selector", err)
		}
		sort.SliceStable(result.Rulings, func(i, j int) bool {
			if result.Rulings[i].PublishedAt == result.Rulings[j].PublishedAt {
				return result.Rulings[i].Comment < result.Rulings[j].Comment
			}
			return result.Rulings[i].PublishedAt > result.Rulings[j].PublishedAt
		})
		result.FetchedAt = result.FetchedAt.UTC()
		return &mcp.CallToolResult{StructuredContent: result}, nil
	})
	return server
}

func exactLookupError(operation, notFoundMessage string, err error) (*mcp.CallToolResult, error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}
	problems := []struct {
		expected      error
		code, message string
	}{
		{card.ErrNotFound, "not_found", notFoundMessage},
		{card.ErrInvalidQuery, "invalid_query", "Scryfall rejected the query"},
		{card.ErrInvalidIdentifier, "invalid_identifier", "the card identifier is invalid"},
		{card.ErrRateLimited, "rate_limited", "the request could not be admitted within its time limit"},
		{card.ErrUpstreamUnavailable, "upstream_unavailable", "Scryfall is temporarily unavailable"},
	}
	for _, problem := range problems {
		if errors.Is(err, problem.expected) {
			return toolError(problem.code, problem.message), nil
		}
	}
	log.Printf("%s failed: %v", operation, err)
	return toolError("internal_error", "an internal error occurred"), nil
}

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var validSetCode = regexp.MustCompile(`^[a-z0-9]{2,8}$`)

func toolError(code, message string) *mcp.CallToolResult {
	b, _ := json.Marshal(map[string]string{"code": code, "message": message})
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}
