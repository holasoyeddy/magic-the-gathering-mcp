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
	return &Cache{inner: inner, dir: dir, now: options.Now, maxBytes: options.MaxBytes, diagnostic: options.Diagnostic, inflight: make(map[string]*flight)}, nil
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

func (c *Cache) get(ctx context.Context, operation, key string, live func() (any, error)) (any, error) {
	if cached, age, ok := c.read(operation, key); ok && age <= freshFor {
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
	}
	return value
}

func (c *Cache) read(operation, key string) (any, time.Duration, bool) {
	path := c.path(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, false
	}
	var record entry
	if json.Unmarshal(data, &record) != nil || record.SchemaVersion != schemaVersion || record.Key != key || record.Operation != operation || record.FetchedAt.IsZero() || record.Integrity != digest(record.Payload) {
		_ = os.Remove(path)
		return nil, 0, false
	}
	var value any
	switch operation {
	case "oracle-card":
		value = &card.OracleCardResult{}
	case "printing":
		value = &card.PrintingResult{}
	default:
		return nil, 0, false
	}
	if json.Unmarshal(record.Payload, value) != nil {
		_ = os.Remove(path)
		return nil, 0, false
	}
	now := c.now().UTC()
	age := now.Sub(record.FetchedAt)
	if age < 0 {
		age = 0
	}
	if age > retainFor {
		_ = os.Remove(path)
		return nil, age, false
	}
	record.LastAccess = now
	if updated, err := json.Marshal(record); err == nil {
		_ = atomicWrite(path, updated)
	}
	switch result := value.(type) {
	case *card.OracleCardResult:
		result.Stale = false
		return *result, age, true
	case *card.PrintingResult:
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
	if err = atomicWrite(c.path(key), data); err != nil {
		return err
	}
	if err = c.evict(); err != nil {
		c.diagnostic("cache cleanup failed: %v", err)
	}
	return nil
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
	files, err := filepath.Glob(filepath.Join(c.dir, "*.json"))
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
		info, e := os.Stat(path)
		if e != nil {
			continue
		}
		data, e := os.ReadFile(path)
		if e != nil {
			continue
		}
		var record entry
		if json.Unmarshal(data, &record) != nil || record.SchemaVersion != schemaVersion {
			_ = os.Remove(path)
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
		if err := os.Remove(entry.path); err != nil {
			return err
		}
		total -= entry.size
	}
	return nil
}
