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

func TestExpectedAdapterFailuresHaveStableToolCodes(t *testing.T) {
	tests := []struct {
		err  error
		code string
	}{
		{card.ErrInvalidQuery, "invalid_query"},
		{card.ErrInvalidIdentifier, "invalid_identifier"},
		{card.ErrRateLimited, "rate_limited"},
		{card.ErrUpstreamUnavailable, "upstream_unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result, err := connect(t, &fakeAdapter{err: fmt.Errorf("private upstream detail: %w", tt.err)}).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_oracle_card", Arguments: map[string]any{"name": "Example"}})
			if err != nil || !result.IsError {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			text := result.Content[0].(*mcp.TextContent).Text
			if strings.Contains(text, "private upstream detail") {
				t.Fatalf("leaked error: %s", text)
			}
			var problem map[string]any
			if json.Unmarshal([]byte(text), &problem) != nil || problem["code"] != tt.code {
				t.Fatalf("error = %q", text)
			}
		})
	}
}

func TestUnexpectedAdapterFailureIsGeneric(t *testing.T) {
	result, err := connect(t, &fakeAdapter{err: errors.New("secret parse failure")}).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_oracle_card", Arguments: map[string]any{"name": "Example"}})
	if err != nil || !result.IsError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if strings.Contains(text, "secret parse failure") || !strings.Contains(text, `"code":"internal_error"`) {
		t.Fatalf("error = %q", text)
	}
}

func TestPrintingFailuresHaveStableSanitizedToolErrors(t *testing.T) {
	tests := []struct {
		name    string
		failure error
		code    string
	}{
		{"not found", card.ErrNotFound, "not_found"},
		{"rate limited", card.ErrRateLimited, "rate_limited"},
		{"upstream unavailable", card.ErrUpstreamUnavailable, "upstream_unavailable"},
		{"unexpected", errors.New("database password in diagnostic"), "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := fmt.Errorf("private upstream detail: %w", tt.failure)
			result, err := connect(t, &fakeAdapter{printingErr: wrapped}).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_printing", Arguments: map[string]any{"scryfall_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}})
			if err != nil || !result.IsError {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			text := result.Content[0].(*mcp.TextContent).Text
			if strings.Contains(text, "private upstream detail") || strings.Contains(text, "database password") {
				t.Fatalf("leaked error: %s", text)
			}
			var problem map[string]any
			if json.Unmarshal([]byte(text), &problem) != nil || problem["code"] != tt.code {
				t.Fatalf("error = %q", text)
			}
		})
	}
}

func TestCancellationPropagatesToAdapter(t *testing.T) {
	adapter := adapterFunc(func(ctx context.Context, _ card.OracleCardSelector) (card.OracleCardResult, error) {
		<-ctx.Done()
		return card.OracleCardResult{}, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := connect(t, adapter).CallTool(ctx, &mcp.CallToolParams{Name: "get_oracle_card", Arguments: map[string]any{"name": "Example"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want canceled", err)
	}
}

type adapterFunc func(context.Context, card.OracleCardSelector) (card.OracleCardResult, error)

func (f adapterFunc) GetOracleCard(ctx context.Context, selector card.OracleCardSelector) (card.OracleCardResult, error) {
	return f(ctx, selector)
}

func (f adapterFunc) GetPrinting(context.Context, card.PrintingSelector) (card.PrintingResult, error) {
	return card.PrintingResult{}, errors.New("unexpected printing call")
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
	if len(result.Tools) != 3 || result.Tools[0].Name != "get_card_legality" || result.Tools[1].Name != "get_oracle_card" || result.Tools[2].Name != "get_printing" {
		t.Fatalf("tools = %#v, want deterministic alphabetical order", result.Tools)
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
