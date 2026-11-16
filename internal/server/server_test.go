package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
	cardserver "github.com/holasoyeddy/magic-the-gathering-mcp/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeAdapter struct {
	result card.OracleCardResult
	err    error
	calls  []card.OracleCardSelector
}

func (f *fakeAdapter) GetOracleCard(_ context.Context, selector card.OracleCardSelector) (card.OracleCardResult, error) {
	f.calls = append(f.calls, selector)
	return f.result, f.err
}

func connect(t *testing.T, adapter card.Adapter) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	serverSession, err := cardserver.New(adapter).Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "contract-test", Version: "v0.0.1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clientSession.Close() })
	return clientSession
}

func TestDiscoveryListsOnlyGetOracleCard(t *testing.T) {
	session := connect(t, &fakeAdapter{})
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 1 || result.Tools[0].Name != "get_oracle_card" {
		t.Fatalf("tools = %#v, want only get_oracle_card", result.Tools)
	}
}

func TestGetOracleCardReturnsCuratedCardAndProvenance(t *testing.T) {
	manaCost, typeLine, oracleText := "{1}{W} // {2}{W}", "Enchantment", "First face text."
	adapter := &fakeAdapter{result: card.OracleCardResult{
		Card: card.OracleCard{
			OracleID: "11111111-1111-4111-8111-111111111111", Name: "Example", Layout: "transform",
			Faces:     []card.OracleCardFace{{Name: "Front", ManaCost: &manaCost, TypeLine: &typeLine, OracleText: &oracleText}, {Name: "Back"}},
			ManaValue: 3, Colors: []string{"W"}, ColorIdentity: []string{"W"}, Keywords: []string{"Transform"},
		},
		SourceURL: "https://api.scryfall.com/cards/named?exact=Example",
		FetchedAt: time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC),
	}}
	session := connect(t, adapter)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_oracle_card", Arguments: map[string]any{"name": "eXaMpLe"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %#v", result.Content)
	}
	if len(adapter.calls) != 1 || adapter.calls[0].Name != "eXaMpLe" {
		t.Fatalf("adapter calls = %#v", adapter.calls)
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var got card.OracleCardResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Card.Name != "Example" || len(got.Card.Faces) != 2 || got.Card.Faces[0].Name != "Front" {
		t.Fatalf("card = %#v", got.Card)
	}
	if got.FetchedAt.Format(time.RFC3339) != "2026-11-16T12:00:00Z" || got.SourceURL == "" || got.Stale {
		t.Fatalf("provenance = %#v", got)
	}
}

func TestGetOracleCardAcceptsOracleID(t *testing.T) {
	adapter := &fakeAdapter{result: card.OracleCardResult{FetchedAt: time.Now()}}
	session := connect(t, adapter)
	id := "11111111-1111-4111-8111-111111111111"
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_oracle_card", Arguments: map[string]any{"oracle_id": id}})
	if err != nil || result.IsError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(adapter.calls) != 1 || adapter.calls[0].OracleID != id {
		t.Fatalf("adapter calls = %#v", adapter.calls)
	}
}

func TestInvalidSelectorsDoNotCallAdapter(t *testing.T) {
	tests := []map[string]any{
		{},
		{"name": "Lightning Bolt", "oracle_id": "11111111-1111-4111-8111-111111111111"},
		{"oracle_id": "not-a-uuid"},
		{"name": ""},
	}
	for _, args := range tests {
		t.Run("invalid", func(t *testing.T) {
			adapter := &fakeAdapter{}
			result, err := connect(t, adapter).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_oracle_card", Arguments: args})
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError || len(adapter.calls) != 0 {
				t.Fatalf("result=%#v calls=%#v", result, adapter.calls)
			}
			text := result.Content[0].(*mcp.TextContent).Text
			var problem map[string]any
			if json.Unmarshal([]byte(text), &problem) != nil || problem["code"] != "invalid_identifier" {
				t.Fatalf("error = %q", text)
			}
		})
	}
}

func TestExactLookupMissIsNotFound(t *testing.T) {
	adapter := &fakeAdapter{err: fmt.Errorf("lookup: %w", card.ErrNotFound)}
	result, err := connect(t, adapter).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_oracle_card", Arguments: map[string]any{"name": "No Such Card"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected tool error")
	}
	text := result.Content[0].(*mcp.TextContent).Text
	var problem map[string]any
	if json.Unmarshal([]byte(text), &problem) != nil || problem["code"] != "not_found" {
		t.Fatalf("error = %q", text)
	}
}
