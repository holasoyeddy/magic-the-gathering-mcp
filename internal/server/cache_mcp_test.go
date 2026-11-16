package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/cardcache"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type synchronizedAdapter struct {
	mu      sync.Mutex
	result  card.OracleCardResult
	err     error
	calls   int
	started chan struct{}
	release chan struct{}
}

type legalityCacheAdapter struct {
	mu     sync.Mutex
	result card.LegalityResult
	err    error
	calls  int
}

func (a *legalityCacheAdapter) GetOracleCard(context.Context, card.OracleCardSelector) (card.OracleCardResult, error) {
	return card.OracleCardResult{}, errors.New("unexpected")
}
func (a *legalityCacheAdapter) GetPrinting(context.Context, card.PrintingSelector) (card.PrintingResult, error) {
	return card.PrintingResult{}, errors.New("unexpected")
}
func (a *legalityCacheAdapter) GetCardLegality(context.Context, card.OracleCardSelector) (card.LegalityResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	return a.result, a.err
}
func (a *legalityCacheAdapter) count() int { a.mu.Lock(); defer a.mu.Unlock(); return a.calls }

func (a *synchronizedAdapter) GetOracleCard(ctx context.Context, _ card.OracleCardSelector) (card.OracleCardResult, error) {
	a.mu.Lock()
	a.calls++
	first := a.calls == 1
	a.mu.Unlock()
	if first && a.started != nil {
		close(a.started)
	}
	if a.release != nil {
		select {
		case <-a.release:
		case <-ctx.Done():
			return card.OracleCardResult{}, ctx.Err()
		}
	}
	return a.result, a.err
}
func (a *synchronizedAdapter) GetPrinting(context.Context, card.PrintingSelector) (card.PrintingResult, error) {
	return card.PrintingResult{}, errors.New("unexpected")
}
func (a *synchronizedAdapter) count() int { a.mu.Lock(); defer a.mu.Unlock(); return a.calls }

func TestMCPFreshHitAndStaleOutageFallback(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	live := &synchronizedAdapter{result: card.OracleCardResult{Card: card.OracleCard{OracleID: "11111111-1111-4111-8111-111111111111", Name: "Example", Faces: []card.OracleCardFace{}, Colors: []string{}, ColorIdentity: []string{}, Keywords: []string{}}, SourceURL: "https://api.scryfall.com/cards/named?exact=Example", FetchedAt: now}}
	cache, err := cardcache.New(live, cardcache.Options{Directory: t.TempDir(), Now: func() time.Time { return now }, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	session := connect(t, cache)
	call := func() card.OracleCardResult {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_oracle_card", Arguments: map[string]any{"name": "Example"}})
		if err != nil || result.IsError {
			t.Fatalf("result=%#v err=%v", result, err)
		}
		b, _ := json.Marshal(result.StructuredContent)
		var got card.OracleCardResult
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := call(); got.Stale || live.count() != 1 {
		t.Fatalf("got=%#v calls=%d", got, live.count())
	}
	now = now.Add(23 * time.Hour)
	if got := call(); got.Stale || live.count() != 1 {
		t.Fatalf("fresh cache got=%#v calls=%d", got, live.count())
	}
	now = now.Add(2 * time.Hour)
	live.err = card.ErrUpstreamUnavailable
	if got := call(); !got.Stale || !got.FetchedAt.Equal(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)) || live.count() != 2 {
		t.Fatalf("stale got=%#v calls=%d", got, live.count())
	}
	now = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC).Add(30*24*time.Hour + time.Second)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_oracle_card", Arguments: map[string]any{"name": "Example"}})
	if err != nil || !result.IsError {
		t.Fatalf("expired stale result=%#v err=%v", result, err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, `"code":"upstream_unavailable"`) {
		t.Fatalf("expired stale error=%s", text)
	}
}

func TestCardLegalityUsesFreshCacheAndStaleOutageFallback(t *testing.T) {
	original := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	now := original
	live := &legalityCacheAdapter{result: card.LegalityResult{
		Legality:  card.CardLegality{OracleID: "11111111-1111-4111-8111-111111111111", Name: "Example", Formats: map[string]card.LegalityStatus{"standard": card.LegalityLegal}},
		SourceURL: "https://api.scryfall.com/cards/named?exact=Example", FetchedAt: original,
	}}
	cache, err := cardcache.New(live, cardcache.Options{Directory: t.TempDir(), Now: func() time.Time { return now }, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	session := connect(t, cache)
	call := func() card.LegalityResult {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_card_legality", Arguments: map[string]any{"name": "Example"}})
		if err != nil || result.IsError {
			t.Fatalf("result=%#v err=%v", result, err)
		}
		b, _ := json.Marshal(result.StructuredContent)
		var got card.LegalityResult
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := call(); got.Stale || live.count() != 1 {
		t.Fatalf("live got=%#v calls=%d", got, live.count())
	}
	now = now.Add(23 * time.Hour)
	if got := call(); got.Stale || live.count() != 1 {
		t.Fatalf("fresh got=%#v calls=%d", got, live.count())
	}
	now = now.Add(2 * time.Hour)
	live.err = card.ErrUpstreamUnavailable
	if got := call(); !got.Stale || !got.FetchedAt.Equal(original) || live.count() != 2 {
		t.Fatalf("stale got=%#v calls=%d", got, live.count())
	}
}

func TestMCPConcurrentIdenticalMissesUseOneUpstreamAttempt(t *testing.T) {
	now := time.Now().UTC()
	live := &synchronizedAdapter{result: card.OracleCardResult{Card: card.OracleCard{Faces: []card.OracleCardFace{}, Colors: []string{}, ColorIdentity: []string{}, Keywords: []string{}}, FetchedAt: now}, started: make(chan struct{}), release: make(chan struct{})}
	cache, err := cardcache.New(live, cardcache.Options{Directory: t.TempDir(), Now: func() time.Time { return now }, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	session := connect(t, cache)
	const count = 6
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		go func() {
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_oracle_card", Arguments: map[string]any{"name": "Example"}})
			if err == nil && result.IsError {
				err = errors.New("tool error")
			}
			errs <- err
		}()
	}
	<-live.started
	time.Sleep(20 * time.Millisecond)
	close(live.release)
	for i := 0; i < count; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if live.count() != 1 {
		t.Fatalf("upstream calls=%d", live.count())
	}
}
