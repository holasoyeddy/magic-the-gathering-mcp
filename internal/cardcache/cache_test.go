package cardcache_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/cardcache"
)

type fake struct {
	mu          sync.Mutex
	oracle      card.OracleCardResult
	oracleErr   error
	printing    card.PrintingResult
	printingErr error
	calls       int
	gate        chan struct{}
}

func (f *fake) GetOracleCard(ctx context.Context, _ card.OracleCardSelector) (card.OracleCardResult, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return card.OracleCardResult{}, ctx.Err()
		}
	}
	return f.oracle, f.oracleErr
}
func (f *fake) GetPrinting(context.Context, card.PrintingSelector) (card.PrintingResult, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return f.printing, f.printingErr
}
func (f *fake) GetRulings(context.Context, card.OracleCardSelector) (card.RulingsResult, error) {
	return card.RulingsResult{}, errors.New("unexpected rulings call")
}

func TestFreshOracleCardSurvivesRestartWithoutUpstreamCall(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	live := &fake{oracle: oracleResult(now)}
	c1 := newCache(t, live, dir, func() time.Time { return now }, 1<<20)
	if _, err := c1.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "Lightning Bolt"}); err != nil {
		t.Fatal(err)
	}
	offline := &fake{oracleErr: card.ErrUpstreamUnavailable}
	c2 := newCache(t, offline, dir, func() time.Time { return now.Add(23 * time.Hour) }, 1<<20)
	got, err := c2.GetOracleCard(context.Background(), card.OracleCardSelector{Name: " lightning bolt "})
	if err != nil || got.Stale || !got.FetchedAt.Equal(now) || offline.calls != 0 {
		t.Fatalf("got=%#v err=%v calls=%d", got, err, offline.calls)
	}
}

func TestDefaultDirectoryHonorsEnvironmentOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "override")
	t.Setenv(cardcache.DirectoryEnvironment, want)
	got, err := cardcache.DefaultDirectory()
	if err != nil || got != want {
		t.Fatalf("directory=%q err=%v", got, err)
	}
}

func TestStaleFallbackPreservesTimestampAndRejectsExpired(t *testing.T) {
	dir := t.TempDir()
	fetched := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	clock := fetched
	c := newCache(t, &fake{oracle: oracleResult(fetched)}, dir, func() time.Time { return clock }, 1<<20)
	selector := card.OracleCardSelector{OracleID: "11111111-1111-4111-8111-111111111111"}
	if _, err := c.GetOracleCard(context.Background(), selector); err != nil {
		t.Fatal(err)
	}
	clock = fetched.Add(25 * time.Hour)
	c = newCache(t, &fake{oracleErr: card.ErrRateLimited}, dir, func() time.Time { return clock }, 1<<20)
	got, err := c.GetOracleCard(context.Background(), selector)
	if err != nil || !got.Stale || !got.FetchedAt.Equal(fetched) {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	clock = fetched.Add(30*24*time.Hour + time.Second)
	c = newCache(t, &fake{oracleErr: card.ErrUpstreamUnavailable}, dir, func() time.Time { return clock }, 1<<20)
	if _, err := c.GetOracleCard(context.Background(), selector); !errors.Is(err, card.ErrUpstreamUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

func TestConcurrentIdenticalMissesCoalesceAndDistinctMissesDoNot(t *testing.T) {
	gate := make(chan struct{})
	live := &fake{oracle: oracleResult(time.Now()), gate: gate}
	c := newCache(t, live, t.TempDir(), time.Now, 1<<20)
	var done atomic.Int32
	for i := 0; i < 8; i++ {
		go func() {
			_, _ = c.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "Same"})
			done.Add(1)
		}()
	}
	time.Sleep(30 * time.Millisecond)
	close(gate)
	deadline := time.Now().Add(time.Second)
	for done.Load() != 8 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if live.calls != 1 {
		t.Fatalf("identical upstream calls=%d", live.calls)
	}
	live2 := &fake{oracle: oracleResult(time.Now())}
	c2 := newCache(t, live2, t.TempDir(), time.Now, 1<<20)
	_, _ = c2.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "A"})
	_, _ = c2.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "B"})
	if live2.calls != 2 {
		t.Fatalf("distinct upstream calls=%d", live2.calls)
	}
}

func TestCorruptAndSchemaIncompatibleEntriesFailSafe(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	live := &fake{oracle: oracleResult(now)}
	c := newCache(t, live, dir, func() time.Time { return now }, 1<<20)
	selector := card.OracleCardSelector{Name: "Example"}
	if _, err := c.GetOracleCard(context.Background(), selector); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) != 1 {
		t.Fatalf("files=%v", files)
	}
	if err := os.WriteFile(files[0], []byte(`{"schema_version":999}`), 0600); err != nil {
		t.Fatal(err)
	}
	freshLive := &fake{oracle: oracleResult(now.Add(time.Hour))}
	c = newCache(t, freshLive, dir, func() time.Time { return now.Add(time.Hour) }, 1<<20)
	got, err := c.GetOracleCard(context.Background(), selector)
	if err != nil || freshLive.calls != 1 || !got.FetchedAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("got=%#v err=%v calls=%d", got, err, freshLive.calls)
	}
	if err := os.WriteFile(files[0], []byte(`not json`), 0600); err != nil {
		t.Fatal(err)
	}
	freshLive.calls = 0
	c = newCache(t, freshLive, dir, func() time.Time { return now.Add(2 * time.Hour) }, 1<<20)
	if _, err := c.GetOracleCard(context.Background(), selector); err != nil || freshLive.calls != 1 {
		t.Fatalf("err=%v calls=%d", err, freshLive.calls)
	}
}

func TestLeastRecentlyUsedEvictionIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	live := &fake{oracle: oracleResult(now)}
	c := newCache(t, live, dir, func() time.Time { return now }, 900)
	_, _ = c.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "A"})
	now = now.Add(time.Second)
	_, _ = c.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "B"})
	now = now.Add(time.Second)
	_, _ = c.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "B"})
	now = now.Add(time.Second)
	_, _ = c.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "C"})
	before := live.calls
	_, _ = c.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "A"})
	if live.calls != before+1 {
		t.Fatalf("least recently used A was not evicted: calls %d -> %d", before, live.calls)
	}
}

func newCache(t *testing.T, inner card.Adapter, dir string, now func() time.Time, max int64) *cardcache.Cache {
	t.Helper()
	c, err := cardcache.New(inner, cardcache.Options{Directory: dir, Now: now, MaxBytes: max})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func oracleResult(at time.Time) card.OracleCardResult {
	return card.OracleCardResult{Card: card.OracleCard{OracleID: "11111111-1111-4111-8111-111111111111", Name: "Example", Faces: []card.OracleCardFace{}, Colors: []string{}, ColorIdentity: []string{}, Keywords: []string{}}, SourceURL: "https://api.scryfall.com/cards/named?exact=Example", FetchedAt: at}
}
