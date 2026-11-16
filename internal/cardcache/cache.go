package cardcache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
)

const (
	schemaVersion        = 1
	defaultMaxBytes      = int64(256 << 20)
	freshFor             = 24 * time.Hour
	retainFor            = 30 * 24 * time.Hour
	DirectoryEnvironment = "MTG_MCP_CACHE_DIR"
)

type Options struct {
	Directory  string
	Now        func() time.Time
	MaxBytes   int64
	Diagnostic func(string, ...any)
}

type Cache struct {
	inner      card.Adapter
	dir        string
	now        func() time.Time
	maxBytes   int64
	diagnostic func(string, ...any)
	mu         sync.Mutex
	inflight   map[string]*flight
	fs         filesystem
}

type filesystem struct {
	readFile    func(string) ([]byte, error)
	stat        func(string) (os.FileInfo, error)
	remove      func(string) error
	glob        func(string) ([]string, error)
	atomicWrite func(string, []byte) error
}

func operatingSystem() filesystem {
	return filesystem{readFile: os.ReadFile, stat: os.Stat, remove: os.Remove, glob: filepath.Glob, atomicWrite: atomicWrite}
}

type flight struct {
	done  chan struct{}
	value any
	err   error
}
type entry struct {
	SchemaVersion int             `json:"schema_version"`
	Key           string          `json:"key"`
	Operation     string          `json:"operation"`
	FetchedAt     time.Time       `json:"fetched_at"`
	LastAccess    time.Time       `json:"last_access"`
	Payload       json.RawMessage `json:"payload"`
	Integrity     string          `json:"integrity"`
}

func DefaultDirectory() (string, error) {
	if override := strings.TrimSpace(os.Getenv(DirectoryEnvironment)); override != "" {
		return override, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	return filepath.Join(base, "magic-the-gathering-mcp"), nil
}

func New(inner card.Adapter, options Options) (*Cache, error) {
	if inner == nil {
		return nil, errors.New("cache requires an adapter")
	}
	dir := options.Directory
	if dir == "" {
		var err error
		dir, err = DefaultDirectory()
		if err != nil {
			return nil, err
		}
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.MaxBytes == 0 {
		options.MaxBytes = defaultMaxBytes
	}
	if options.MaxBytes < 1 {
		return nil, errors.New("cache maximum size must be positive")
	}
	if options.Diagnostic == nil {
		options.Diagnostic = func(string, ...any) {}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	c := &Cache{inner: inner, dir: dir, now: options.Now, maxBytes: options.MaxBytes, diagnostic: options.Diagnostic, inflight: make(map[string]*flight), fs: operatingSystem()}
	c.cleanup()
	return c, nil
}

func (c *Cache) GetOracleCard(ctx context.Context, selector card.OracleCardSelector) (card.OracleCardResult, error) {
	key := oracleKey(selector)
	value, err := c.get(ctx, "oracle-card", key, func() (any, error) { return c.inner.GetOracleCard(ctx, selector) })
	if err != nil {
		return card.OracleCardResult{}, err
	}
	return value.(card.OracleCardResult), nil
}

func (c *Cache) GetPrinting(ctx context.Context, selector card.PrintingSelector) (card.PrintingResult, error) {
	key := printingKey(selector)
	value, err := c.get(ctx, "printing", key, func() (any, error) { return c.inner.GetPrinting(ctx, selector) })
	if err != nil {
		return card.PrintingResult{}, err
	}
	return value.(card.PrintingResult), nil
}

func (c *Cache) GetCardLegality(ctx context.Context, selector card.OracleCardSelector) (card.LegalityResult, error) {
	key := legalityKey(selector)
	value, err := c.get(ctx, "card-legality", key, func() (any, error) { return c.inner.GetCardLegality(ctx, selector) })
	if err != nil {
		return card.LegalityResult{}, err
	}
	return value.(card.LegalityResult), nil
}

func (c *Cache) GetRulings(ctx context.Context, selector card.OracleCardSelector) (card.RulingsResult, error) {
	key := rulingsKey(selector)
	value, err := c.get(ctx, "rulings", key, func() (any, error) { return c.inner.GetRulings(ctx, selector) })
	if err != nil {
		return card.RulingsResult{}, err
	}
	return value.(card.RulingsResult), nil
}

func oracleKey(s card.OracleCardSelector) string {
	if s.OracleID != "" {
		return "v1:oracle-card:oracle-id:" + strings.ToLower(strings.TrimSpace(s.OracleID))
	}
	return "v1:oracle-card:exact-name:" + strings.ToLower(strings.TrimSpace(s.Name))
}
func printingKey(s card.PrintingSelector) string {
	if s.ScryfallID != "" {
		return "v1:printing:scryfall-id:" + strings.ToLower(strings.TrimSpace(s.ScryfallID))
	}
	return "v1:printing:catalog:" + strings.ToLower(strings.TrimSpace(s.SetCode)) + ":" + strings.TrimSpace(s.CollectorNumber)
}
func legalityKey(s card.OracleCardSelector) string {
	if s.OracleID != "" {
		return "v1:card-legality:oracle-id:" + strings.ToLower(strings.TrimSpace(s.OracleID))
	}
	return "v1:card-legality:exact-name:" + strings.ToLower(strings.TrimSpace(s.Name))
}
func rulingsKey(s card.OracleCardSelector) string {
	if s.OracleID != "" {
		return "v1:rulings:oracle-id:" + strings.ToLower(strings.TrimSpace(s.OracleID))
	}
	return "v1:rulings:exact-name:" + strings.ToLower(strings.TrimSpace(s.Name))
}

func (c *Cache) get(ctx context.Context, operation, key string, live func() (any, error)) (any, error) {
	if cached, age, ok := c.read(operation, key); ok && age <= freshFor {
		c.cleanup()
		return cached, nil
	}
	c.mu.Lock()
	if existing := c.inflight[key]; existing != nil {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-existing.done:
			return existing.value, existing.err
		}
	}
	f := &flight{done: make(chan struct{})}
	c.inflight[key] = f
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.inflight, key); close(f.done); c.mu.Unlock() }()
	// Re-read after winning the flight because another completed request may have populated disk.
	if cached, age, ok := c.read(operation, key); ok && age <= freshFor {
		c.cleanup()
		f.value = cached
		return cached, nil
	}
	value, err := live()
	if err == nil {
		if writeErr := c.write(operation, key, value); writeErr != nil {
			c.diagnostic("cache write failed: %v", writeErr)
		}
		f.value = value
		return value, nil
	}
	if errors.Is(err, card.ErrRateLimited) || errors.Is(err, card.ErrUpstreamUnavailable) {
		if cached, age, ok := c.read(operation, key); ok && age <= retainFor {
			cached = markStale(cached)
			f.value = cached
			return cached, nil
		}
	}
	f.err = err
	return nil, err
}

func markStale(value any) any {
	switch result := value.(type) {
	case card.OracleCardResult:
		result.Stale = true
		return result
	case card.PrintingResult:
		result.Stale = true
		return result
	case card.LegalityResult:
		result.Stale = true
		return result
	case card.RulingsResult:
		result.Stale = true
		return result
	}
	return value
}

func (c *Cache) read(operation, key string) (any, time.Duration, bool) {
	path := c.path(key)
	data, err := c.fs.readFile(path)
	if err != nil {
		return nil, 0, false
	}
	var record entry
	if json.Unmarshal(data, &record) != nil || record.SchemaVersion != schemaVersion || record.Key != key || record.Operation != operation || record.FetchedAt.IsZero() || record.Integrity != digest(record.Payload) {
		c.removeInvalid(path, "corrupt or incompatible cache entry")
		return nil, 0, false
	}
	var value any
	switch operation {
	case "oracle-card":
		value = &card.OracleCardResult{}
	case "printing":
		value = &card.PrintingResult{}
	case "card-legality":
		value = &card.LegalityResult{}
	case "rulings":
		value = &card.RulingsResult{}
	default:
		return nil, 0, false
	}
	if json.Unmarshal(record.Payload, value) != nil {
		c.removeInvalid(path, "corrupt cache payload")
		return nil, 0, false
	}
	now := c.now().UTC()
	age := now.Sub(record.FetchedAt)
	if age < 0 {
		age = 0
	}
	if age > retainFor {
		c.removeInvalid(path, "expired cache entry")
		return nil, age, false
	}
	record.LastAccess = now
	if updated, err := json.Marshal(record); err == nil {
		if err := c.fs.atomicWrite(path, updated); err != nil {
			c.diagnostic("cache last-access update failed for %s: %v", path, err)
		}
	}
	switch result := value.(type) {
	case *card.OracleCardResult:
		result.Stale = false
		return *result, age, true
	case *card.PrintingResult:
		result.Stale = false
		return *result, age, true
	case *card.LegalityResult:
		result.Stale = false
		return *result, age, true
	case *card.RulingsResult:
		result.Stale = false
		return *result, age, true
	}
	return nil, 0, false
}

func (c *Cache) write(operation, key string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	fetched := c.now().UTC()
	switch result := value.(type) {
	case card.OracleCardResult:
		fetched = result.FetchedAt.UTC()
	case card.PrintingResult:
		fetched = result.FetchedAt.UTC()
	case card.LegalityResult:
		fetched = result.FetchedAt.UTC()
	case card.RulingsResult:
		fetched = result.FetchedAt.UTC()
	}
	if fetched.IsZero() {
		fetched = c.now().UTC()
	}
	record := entry{SchemaVersion: schemaVersion, Key: key, Operation: operation, FetchedAt: fetched, LastAccess: c.now().UTC(), Payload: payload}
	record.Integrity = digest(payload)
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err = c.fs.atomicWrite(c.path(key), data); err != nil {
		return err
	}
	c.cleanup()
	return nil
}

func (c *Cache) removeInvalid(path, reason string) {
	if err := c.fs.remove(path); err != nil {
		c.diagnostic("%s removal failed for %s: %v", reason, path, err)
	}
}

func (c *Cache) cleanup() {
	if err := c.evict(); err != nil {
		c.diagnostic("cache cleanup failed: %v", err)
	}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (c *Cache) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:])+".json")
}
func atomicWrite(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".cache-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err = temp.Chmod(0600); err == nil {
		_, err = temp.Write(data)
	}
	closeErr := temp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}

func (c *Cache) evict() error {
	files, err := c.fs.glob(filepath.Join(c.dir, "*.json"))
	if err != nil {
		return err
	}
	type item struct {
		path   string
		size   int64
		access time.Time
	}
	items := make([]item, 0, len(files))
	var total int64
	for _, path := range files {
		info, e := c.fs.stat(path)
		if e != nil {
			if removeErr := c.fs.remove(path); removeErr != nil {
				return fmt.Errorf("account for unreadable cache entry %s: stat: %v; remove: %w", path, e, removeErr)
			}
			continue
		}
		data, e := c.fs.readFile(path)
		if e != nil {
			if removeErr := c.fs.remove(path); removeErr != nil {
				return fmt.Errorf("account for unreadable cache entry %s (%d bytes): read: %v; remove: %w", path, info.Size(), e, removeErr)
			}
			continue
		}
		var record entry
		if json.Unmarshal(data, &record) != nil || record.SchemaVersion != schemaVersion || record.Key == "" || record.Operation == "" || record.FetchedAt.IsZero() || record.Integrity != digest(record.Payload) {
			if removeErr := c.fs.remove(path); removeErr != nil {
				return fmt.Errorf("remove corrupt cache entry %s (%d bytes): %w", path, info.Size(), removeErr)
			}
			continue
		}
		items = append(items, item{path, info.Size(), record.LastAccess})
		total += info.Size()
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].access.Equal(items[j].access) {
			return items[i].path < items[j].path
		}
		return items[i].access.Before(items[j].access)
	})
	for _, entry := range items {
		if total <= c.maxBytes {
			break
		}
		if err := c.fs.remove(entry.path); err != nil {
			return err
		}
		total -= entry.size
	}
	return nil
}
