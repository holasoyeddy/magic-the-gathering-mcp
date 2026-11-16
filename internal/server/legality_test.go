package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type legalityAdapter struct {
	fakeAdapter
	result card.LegalityResult
	err    error
	calls  []card.OracleCardSelector
}

func (a *legalityAdapter) GetCardLegality(_ context.Context, selector card.OracleCardSelector) (card.LegalityResult, error) {
	a.calls = append(a.calls, selector)
	return a.result, a.err
}

func TestDiscoveryIncludesCardLegalityAfterExactLookupTools(t *testing.T) {
	result, err := connect(t, &legalityAdapter{}).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"get_card_legality", "get_oracle_card", "get_printing", "get_rulings"}
	if len(result.Tools) != len(want) {
		t.Fatalf("tools = %#v", result.Tools)
	}
	for i, name := range want {
		if result.Tools[i].Name != name {
			t.Fatalf("tool %d = %q, want %q", i, result.Tools[i].Name, name)
		}
	}
}

func TestGetCardLegalityByNameReturnsEveryUpstreamStatusAndProvenance(t *testing.T) {
	fetched := time.Date(2026, 11, 16, 18, 0, 0, 0, time.FixedZone("CST", -6*60*60))
	adapter := &legalityAdapter{result: card.LegalityResult{
		Legality: card.CardLegality{OracleID: "11111111-1111-4111-8111-111111111111", Name: "Example", Formats: map[string]card.LegalityStatus{
			"standard": card.LegalityLegal, "future": card.LegalityNotLegal, "vintage": card.LegalityRestricted, "legacy": card.LegalityBanned,
		}},
		SourceURL: "https://api.scryfall.com/cards/named?exact=Example", FetchedAt: fetched,
	}}
	result, err := connect(t, adapter).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_card_legality", Arguments: map[string]any{"name": "eXaMpLe"}})
	if err != nil || result.IsError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(adapter.calls) != 1 || adapter.calls[0].Name != "eXaMpLe" {
		t.Fatalf("calls = %#v", adapter.calls)
	}
	b, _ := json.Marshal(result.StructuredContent)
	var got card.LegalityResult
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Legality.Formats) != 4 || got.Legality.Formats["standard"] != card.LegalityLegal || got.Legality.Formats["future"] != card.LegalityNotLegal || got.Legality.Formats["vintage"] != card.LegalityRestricted || got.Legality.Formats["legacy"] != card.LegalityBanned {
		t.Fatalf("legality = %#v", got.Legality)
	}
	if got.FetchedAt.Location() != time.UTC || got.Stale || got.SourceURL == "" {
		t.Fatalf("provenance = %#v", got)
	}
}

func TestGetCardLegalityByOracleIDUsesExactSelector(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	adapter := &legalityAdapter{}
	result, err := connect(t, adapter).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_card_legality", Arguments: map[string]any{"oracle_id": id}})
	if err != nil || result.IsError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(adapter.calls) != 1 || adapter.calls[0].OracleID != id {
		t.Fatalf("calls = %#v", adapter.calls)
	}
}

func TestGetCardLegalityRejectsInvalidSelectorsWithoutCallingAdapter(t *testing.T) {
	tests := []map[string]any{{}, {"name": "Example", "oracle_id": "11111111-1111-4111-8111-111111111111"}, {"oracle_id": "not-a-uuid"}, {"name": " "}, {"deck": []string{"Example"}}}
	for _, args := range tests {
		adapter := &legalityAdapter{}
		result, err := connect(t, adapter).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_card_legality", Arguments: args})
		if err != nil || !result.IsError || len(adapter.calls) != 0 {
			t.Fatalf("args=%#v result=%#v err=%v calls=%#v", args, result, err, adapter.calls)
		}
		if text := result.Content[0].(*mcp.TextContent).Text; !strings.Contains(text, `"code":"invalid_identifier"`) {
			t.Fatalf("error = %s", text)
		}
	}
}

func TestGetCardLegalityMapsExactMissAndUpstreamFailures(t *testing.T) {
	for _, tt := range []struct {
		failure error
		code    string
	}{
		{card.ErrNotFound, "not_found"}, {card.ErrRateLimited, "rate_limited"}, {card.ErrUpstreamUnavailable, "upstream_unavailable"}, {errors.New("secret"), "internal_error"},
	} {
		result, err := connect(t, &legalityAdapter{err: fmt.Errorf("private: %w", tt.failure)}).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_card_legality", Arguments: map[string]any{"name": "Missing"}})
		if err != nil || !result.IsError {
			t.Fatalf("result=%#v err=%v", result, err)
		}
		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, `"code":"`+tt.code+`"`) || strings.Contains(text, "private") || strings.Contains(text, "secret") {
			t.Fatalf("error = %s", text)
		}
	}
}
