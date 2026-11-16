package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/cardcache"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type searchFake struct {
	fakeAdapter
	result card.SearchResult
	err    error
	calls  []card.SearchRequest
}

func (f *searchFake) SearchCards(_ context.Context, request card.SearchRequest) (card.SearchResult, error) {
	f.calls = append(f.calls, request)
	return f.result, f.err
}

func TestSearchCardsDiscoveryAndDefaults(t *testing.T) {
	adapter := &searchFake{result: card.SearchResult{Results: []card.SearchSummary{}, SourceURL: "https://api.scryfall.test/cards/search?q=t%3Agoblin", FetchedAt: time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)}}
	session := connect(t, adapter)
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 4 || listed.Tools[0].Name != "get_oracle_card" || listed.Tools[1].Name != "get_printing" || listed.Tools[2].Name != "get_rulings" || listed.Tools[3].Name != "search_cards" {
		t.Fatalf("tools = %#v", listed.Tools)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "search_cards", Arguments: map[string]any{"query": " t:goblin "}})
	if err != nil || result.IsError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(adapter.calls) != 1 || adapter.calls[0].Query != "t:goblin" || adapter.calls[0].ResultMode != card.SearchOracleCards || adapter.calls[0].PageSize != 20 {
		t.Fatalf("calls = %#v", adapter.calls)
	}
}

func TestSearchCardsReturnsCompactModesAndCursor(t *testing.T) {
	for _, mode := range []card.SearchResultMode{card.SearchOracleCards, card.SearchPrintings} {
		t.Run(string(mode), func(t *testing.T) {
			next := card.SearchContinuation{Position: "https://api.scryfall.test/cards/search?page=2"}
			adapter := &searchFake{result: card.SearchResult{Results: []card.SearchSummary{{OracleID: "11111111-1111-4111-8111-111111111111", ScryfallID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Name: "Goblin Guide", ManaCost: "{R}", TypeLine: "Creature — Goblin Scout"}}, Next: &next, SourceURL: "https://api.scryfall.test/cards/search?q=t%3Agoblin", FetchedAt: time.Now()}}
			result, err := connect(t, adapter).CallTool(context.Background(), &mcp.CallToolParams{Name: "search_cards", Arguments: map[string]any{"query": "t:goblin", "result_mode": string(mode), "page_size": 1}})
			if err != nil || result.IsError {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			b, _ := json.Marshal(result.StructuredContent)
			var got map[string]any
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatal(err)
			}
			if got["next_cursor"] == nil || strings.Contains(fmt.Sprint(got["next_cursor"]), "scryfall") {
				t.Fatalf("cursor = %#v", got["next_cursor"])
			}
			if strings.Contains(string(b), "oracle_text") || strings.Contains(string(b), "image_urls") {
				t.Fatalf("unbounded summary: %s", b)
			}
		})
	}
}

func TestSearchCardsValidatesBoundsAndCursorBinding(t *testing.T) {
	adapter := &searchFake{result: card.SearchResult{Results: []card.SearchSummary{}, FetchedAt: time.Now()}}
	session := connect(t, adapter)
	for _, size := range []int{0, 101} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "search_cards", Arguments: map[string]any{"query": "x", "page_size": size}})
		if err != nil || !result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "invalid_identifier") {
			t.Fatalf("size=%d result=%#v err=%v", size, result, err)
		}
	}
	first, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "search_cards", Arguments: map[string]any{"query": "x", "page_size": 2}})
	if err != nil || first.IsError {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	adapter.result.Next = &card.SearchContinuation{Position: "page-2"}
	withCursor, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "search_cards", Arguments: map[string]any{"query": "x", "page_size": 2}})
	if err != nil || withCursor.IsError {
		t.Fatal(err)
	}
	b, _ := json.Marshal(withCursor.StructuredContent)
	var payload map[string]any
	_ = json.Unmarshal(b, &payload)
	cursor := payload["next_cursor"].(string)
	for _, args := range []map[string]any{{"query": "y", "page_size": 2, "cursor": cursor}, {"query": "x", "page_size": 3, "cursor": cursor}, {"query": "x", "page_size": 2, "cursor": "broken"}} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "search_cards", Arguments: args})
		if err != nil || !result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "invalid_identifier") {
			t.Fatalf("args=%#v result=%#v err=%v", args, result, err)
		}
	}
}

func TestSearchCardsMapsInvalidQueryWithoutLeakingBody(t *testing.T) {
	adapter := &searchFake{err: fmt.Errorf("secret upstream body: %w", card.ErrInvalidQuery)}
	result, err := connect(t, adapter).CallTool(context.Background(), &mcp.CallToolParams{Name: "search_cards", Arguments: map[string]any{"query": "bad"}})
	if err != nil || !result.IsError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "invalid_query") || strings.Contains(text, "secret") {
		t.Fatalf("error=%s", text)
	}
}

func TestSearchCardsUsesFreshCacheAndStaleFallback(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	live := &searchFake{result: card.SearchResult{Results: []card.SearchSummary{{Name: "Cached Goblin"}}, SourceURL: "https://api.scryfall.test/cards/search?q=goblin", FetchedAt: now}}
	cache, err := cardcache.New(live, cardcache.Options{Directory: t.TempDir(), Now: func() time.Time { return now }, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	session := connect(t, cache)
	call := func() card.SearchResult {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "search_cards", Arguments: map[string]any{"query": "goblin", "page_size": 5}})
		if err != nil || result.IsError {
			t.Fatalf("result=%#v err=%v", result, err)
		}
		b, _ := json.Marshal(result.StructuredContent)
		var got card.SearchResult
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	_ = call()
	_ = call()
	if len(live.calls) != 1 {
		t.Fatalf("fresh calls=%d", len(live.calls))
	}
	now = now.Add(25 * time.Hour)
	live.err = card.ErrUpstreamUnavailable
	got := call()
	if !got.Stale || got.FetchedAt != live.result.FetchedAt || len(live.calls) != 2 {
		t.Fatalf("stale=%#v calls=%d", got, len(live.calls))
	}
}
