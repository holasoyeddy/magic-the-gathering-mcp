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
	result         card.OracleCardResult
	err            error
	calls          []card.OracleCardSelector
	printingResult card.PrintingResult
	printingErr    error
	printingCalls  []card.PrintingSelector
}

func (f *fakeAdapter) GetPrinting(_ context.Context, selector card.PrintingSelector) (card.PrintingResult, error) {
	f.printingCalls = append(f.printingCalls, selector)
	return f.printingResult, f.printingErr
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

func TestDiscoveryListsExactLookupToolsInDeterministicOrder(t *testing.T) {
	session := connect(t, &fakeAdapter{})
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 2 || result.Tools[0].Name != "get_oracle_card" || result.Tools[1].Name != "get_printing" {
		t.Fatalf("tools = %#v, want get_oracle_card then get_printing", result.Tools)
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

func TestGetPrintingBySetAndCollectorReturnsLocalizedMultiFaceFacts(t *testing.T) {
	oracleID := "11111111-1111-4111-8111-111111111111"
	printedName, printedType, printedText, faceName := "Ejemplo", "Criatura", "Texto.", "Frente"
	adapter := &fakeAdapter{printingResult: card.PrintingResult{
		Printing: card.Printing{
			ScryfallID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", OracleID: &oracleID,
			Name: "Example // Reverse", Language: "es", PrintedName: &printedName,
			PrintedTypeLine: &printedType, PrintedText: &printedText, SetCode: "neo", SetName: "Kamigawa: Neon Dynasty",
			CollectorNumber: "123a", Rarity: "rare", ReleasedAt: "2022-02-18", Finishes: []string{"nonfoil", "foil"},
			Frame: "2015", FrameEffects: []string{"showcase"}, BorderColor: "black", FullArt: true,
			Faces: []card.PrintingFace{{Name: "Example", PrintedName: &faceName, ImageURLs: map[string]string{"normal": "https://cards.scryfall.io/normal/front.jpg"}}, {Name: "Reverse"}},
		}, SourceURL: "https://api.scryfall.com/cards/neo/123a", FetchedAt: time.Date(2026, 11, 16, 12, 0, 0, 0, time.FixedZone("test", -6*60*60)),
	}}
	result, err := connect(t, adapter).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_printing", Arguments: map[string]any{"set_code": " NEO ", "collector_number": "123a"}})
	if err != nil || result.IsError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(adapter.printingCalls) != 1 || adapter.printingCalls[0].SetCode != "neo" || adapter.printingCalls[0].CollectorNumber != "123a" {
		t.Fatalf("calls = %#v", adapter.printingCalls)
	}
	b, _ := json.Marshal(result.StructuredContent)
	var got card.PrintingResult
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Printing.Language != "es" || got.Printing.PrintedName == nil || *got.Printing.PrintedName != "Ejemplo" || len(got.Printing.Faces) != 2 || got.Printing.Faces[0].Name != "Example" {
		t.Fatalf("printing = %#v", got.Printing)
	}
	var public map[string]any
	if err := json.Unmarshal(b, &public); err != nil {
		t.Fatal(err)
	}
	printing := public["printing"].(map[string]any)
	if got.Printing.OracleID == nil || printing["oracle_text"] != nil || printing["mana_value"] != nil || printing["color_identity"] != nil {
		t.Fatal("printing must reference Oracle identity without embedding Oracle card fields")
	}
	if got.FetchedAt.Format(time.RFC3339) != "2026-11-16T18:00:00Z" {
		t.Fatalf("fetched_at = %s", got.FetchedAt)
	}
}

func TestGetPrintingByScryfallID(t *testing.T) {
	id := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	adapter := &fakeAdapter{printingResult: card.PrintingResult{FetchedAt: time.Now()}}
	result, err := connect(t, adapter).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_printing", Arguments: map[string]any{"scryfall_id": id}})
	if err != nil || result.IsError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(adapter.printingCalls) != 1 || adapter.printingCalls[0].ScryfallID != id {
		t.Fatalf("calls = %#v", adapter.printingCalls)
	}
}

func TestInvalidPrintingSelectorsDoNotCallAdapter(t *testing.T) {
	tests := []map[string]any{{}, {"scryfall_id": "bad"}, {"set_code": "neo"}, {"collector_number": "123a"}, {"set_code": "!", "collector_number": "1"}, {"scryfall_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "set_code": "neo", "collector_number": "1"}}
	for _, args := range tests {
		adapter := &fakeAdapter{}
		result, err := connect(t, adapter).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_printing", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError || len(adapter.printingCalls) != 0 {
			t.Fatalf("args=%#v result=%#v calls=%#v", args, result, adapter.printingCalls)
		}
	}
}
