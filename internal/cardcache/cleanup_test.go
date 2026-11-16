package cardcache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
)

type cleanupAdapter struct {
	calls int
	now   func() time.Time
}

func (a *cleanupAdapter) GetOracleCard(context.Context, card.OracleCardSelector) (card.OracleCardResult, error) {
	a.calls++
	return card.OracleCardResult{Card: card.OracleCard{Name: "Example", Faces: []card.OracleCardFace{}, Colors: []string{}, ColorIdentity: []string{}, Keywords: []string{}}, FetchedAt: a.now()}, nil
}
func (*cleanupAdapter) GetPrinting(context.Context, card.PrintingSelector) (card.PrintingResult, error) {
	return card.PrintingResult{}, errors.New("unexpected")
}
func (*cleanupAdapter) GetRulings(context.Context, card.OracleCardSelector) (card.RulingsResult, error) {
	return card.RulingsResult{}, errors.New("unexpected")
}
func (*cleanupAdapter) GetCardLegality(context.Context, card.OracleCardSelector) (card.LegalityResult, error) {
	return card.LegalityResult{}, errors.New("unexpected legality call")
}

func TestStartupAndFreshHitsEnforceCapacity(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	live := &cleanupAdapter{now: func() time.Time { return now }}
	large, err := New(live, Options{Directory: dir, Now: func() time.Time { return now }, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"A", "B", "C"} {
		if _, err := large.GetOracleCard(context.Background(), card.OracleCardSelector{Name: name}); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Second)
	}
	bounded, err := New(live, Options{Directory: dir, Now: func() time.Time { return now }, MaxBytes: 900})
	if err != nil {
		t.Fatal(err)
	}
	if size := cacheBytes(t, dir); size > 900 {
		t.Fatalf("startup cache size=%d", size)
	}
	// Add a valid entry after startup, then prove a fresh-only lookup also restores the bound.
	extraDir := t.TempDir()
	extra, err := New(live, Options{Directory: extraDir, Now: func() time.Time { return now }, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := extra.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "D"}); err != nil {
		t.Fatal(err)
	}
	extraFiles, _ := filepath.Glob(filepath.Join(extraDir, "*.json"))
	data, err := os.ReadFile(extraFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.Base(extraFiles[0])), data, 0600); err != nil {
		t.Fatal(err)
	}
	before := live.calls
	if _, err := bounded.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "C"}); err != nil {
		t.Fatal(err)
	}
	if live.calls != before {
		t.Fatal("fresh hit called upstream")
	}
	if size := cacheBytes(t, dir); size > 900 {
		t.Fatalf("fresh-hit cache size=%d", size)
	}
}

func TestCleanupDoesNotSilentlyUndercountProblematicEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	diagnostics := []string{}
	c := &Cache{dir: dir, maxBytes: 1, diagnostic: func(format string, args ...any) { diagnostics = append(diagnostics, fmt.Sprintf(format, args...)) }, fs: operatingSystem()}
	c.fs.stat = func(string) (os.FileInfo, error) { return nil, errors.New("stat denied") }
	c.fs.remove = func(string) error { return errors.New("remove denied") }
	c.cleanup()
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "stat denied") || !strings.Contains(diagnostics[0], "remove denied") {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	c.fs = operatingSystem()
	c.fs.readFile = func(string) ([]byte, error) { return nil, errors.New("read denied") }
	c.fs.remove = func(string) error { return errors.New("remove denied") }
	diagnostics = nil
	c.cleanup()
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "1 bytes") && !strings.Contains(diagnostics[0], "3 bytes") {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
}

func TestReadCleanupFailuresAreDiagnostic(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	live := &cleanupAdapter{now: func() time.Time { return now }}
	messages := []string{}
	c, err := New(live, Options{Directory: dir, Now: func() time.Time { return now }, MaxBytes: 1 << 20, Diagnostic: func(format string, args ...any) { messages = append(messages, fmt.Sprintf(format, args...)) }})
	if err != nil {
		t.Fatal(err)
	}
	selector := card.OracleCardSelector{Name: "Example"}
	if _, err := c.GetOracleCard(context.Background(), selector); err != nil {
		t.Fatal(err)
	}
	path := c.path(oracleKey(selector))
	real := operatingSystem()
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c.fs.atomicWrite = func(string, []byte) error { return errors.New("write denied") }
	if _, _, ok := c.read("oracle-card", oracleKey(selector)); !ok {
		t.Fatal("fresh entry missing")
	}
	if !contains(messages, "last-access update failed") {
		t.Fatalf("messages=%v", messages)
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	c.fs.atomicWrite = real.atomicWrite
	c.fs.remove = func(string) error { return errors.New("remove denied") }
	c.read("oracle-card", oracleKey(selector))
	if !contains(messages, "corrupt or incompatible cache entry removal failed") {
		t.Fatalf("messages=%v", messages)
	}
	// Eviction must also report a corrupt-entry removal failure.
	c.cleanup()
	if !contains(messages, "remove corrupt cache entry") {
		t.Fatalf("messages=%v", messages)
	}
	messages = nil
	var schemaRecord entry
	if err := json.Unmarshal(original, &schemaRecord); err != nil {
		t.Fatal(err)
	}
	schemaRecord.SchemaVersion = 999
	schemaData, _ := json.Marshal(schemaRecord)
	if err := os.WriteFile(path, schemaData, 0600); err != nil {
		t.Fatal(err)
	}
	c.read("oracle-card", oracleKey(selector))
	if !contains(messages, "corrupt or incompatible cache entry removal failed") {
		t.Fatalf("schema removal messages=%v", messages)
	}
	messages = nil
	var expiredRecord entry
	if err := json.Unmarshal(original, &expiredRecord); err != nil {
		t.Fatal(err)
	}
	expiredRecord.FetchedAt = now.Add(-retainFor - time.Second)
	expiredData, _ := json.Marshal(expiredRecord)
	if err := os.WriteFile(path, expiredData, 0600); err != nil {
		t.Fatal(err)
	}
	c.read("oracle-card", oracleKey(selector))
	if !contains(messages, "expired cache entry removal failed") {
		t.Fatalf("expired removal messages=%v", messages)
	}
}

func cacheBytes(t *testing.T, dir string) int64 {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	var total int64
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		total += info.Size()
	}
	return total
}
func contains(values []string, part string) bool {
	for _, value := range values {
		if strings.Contains(value, part) {
			return true
		}
	}
	return false
}
