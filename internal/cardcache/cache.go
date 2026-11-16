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
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
)

const (
	schemaVersion    = 1
	defaultMaxBytes  = int64(256 << 20)
	freshFor         = 24 * time.Hour
	defaultRetainFor = 30 * 24 * time.Hour
	maximumRetainFor = 30 * 24 * time.Hour
	// DirectoryEnvironment names the optional cache-directory override.
	DirectoryEnvironment = "MTG_MCP_CACHE_DIR"
)

// Options configures persistent card-fact caching. Zero values select the
// documented defaults. RetainFor is capped at 30 days, and Diagnostic receives
// best-effort maintenance errors that do not prevent a card-fact response.
type Options struct {
	Directory  string
	Now        func() time.Time
	MaxBytes   int64
	RetainFor  time.Duration
	Diagnostic func(string, ...any)
}

// Cache is a concurrency-safe, persistent card-fact adapter. It coalesces
// identical live requests and may return an explicitly stale result when a
// refresh is rate limited or unavailable within the configured retention window.
type Cache struct {
	inner      card.Adapter
	dir        string
	now        func() time.Time
	maxBytes   int64
	retainFor  time.Duration
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
	Operation     operation       `json:"operation"`
	FetchedAt     time.Time       `json:"fetched_at"`
	LastAccess    time.Time       `json:"last_access"`
	Payload       json.RawMessage `json:"payload"`
	Integrity     string          `json:"integrity"`
}

type operation string

const (
	operationOracle   operation = "oracle-card"
	operationPrinting operation = "printing"
	operationLegality operation = "card-legality"
	operationSearch   operation = "card-search"
	operationRulings  operation = "rulings"
)

// DefaultDirectory returns the configured cache directory, or an application
// directory beneath the current user's cache directory when no override exists.
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

// New creates a persistent Cache around inner and performs best-effort startup
// maintenance. It creates the cache directory and returns an error when the
// configuration or required directory setup is invalid.
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
	if options.RetainFor == 0 {
		options.RetainFor = defaultRetainFor
	}
	if options.RetainFor < 1 {
		return nil, errors.New("cache retention must be positive")
	}
	if options.RetainFor > maximumRetainFor {
		options.RetainFor = maximumRetainFor
	}
	if options.Diagnostic == nil {
		options.Diagnostic = func(string, ...any) {}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	c := &Cache{inner: inner, dir: dir, now: options.Now, maxBytes: options.MaxBytes, retainFor: options.RetainFor, diagnostic: options.Diagnostic, inflight: make(map[string]*flight), fs: operatingSystem()}
	c.cleanup()
	return c, nil
}

// GetOracleCard returns an exact Oracle-card fact, reusing a fresh cached result
// or an eligible stale result when the live adapter is unavailable.
func (c *Cache) GetOracleCard(ctx context.Context, selector card.OracleCardSelector) (card.OracleCardResult, error) {
	key := oracleKey(selector)
	value, err := c.get(ctx, operationOracle, key, func() (any, error) { return c.inner.GetOracleCard(ctx, selector) })
	if err != nil {
		return card.OracleCardResult{}, err
	}
	return value.(card.OracleCardResult), nil
}

// GetPrinting returns an exact printing fact under the cache freshness policy.
func (c *Cache) GetPrinting(ctx context.Context, selector card.PrintingSelector) (card.PrintingResult, error) {
	key := printingKey(selector)
	value, err := c.get(ctx, operationPrinting, key, func() (any, error) { return c.inner.GetPrinting(ctx, selector) })
	if err != nil {
		return card.PrintingResult{}, err
	}
	return value.(card.PrintingResult), nil
}

// GetCardLegality returns card legality under the cache freshness policy.
func (c *Cache) GetCardLegality(ctx context.Context, selector card.OracleCardSelector) (card.LegalityResult, error) {
	key := legalityKey(selector)
	value, err := c.get(ctx, operationLegality, key, func() (any, error) { return c.inner.GetCardLegality(ctx, selector) })
	if err != nil {
		return card.LegalityResult{}, err
	}
	return value.(card.LegalityResult), nil
}

// SearchCards returns an ordered page of card-search results under the cache
// freshness policy. It fails when the wrapped adapter cannot search.
func (c *Cache) SearchCards(ctx context.Context, request card.SearchRequest) (card.SearchResult, error) {
	searcher, ok := c.inner.(card.Searcher)
	if !ok {
		return card.SearchResult{}, errors.New("adapter does not support card search")
	}
	key := searchKey(request)
	value, err := c.get(ctx, operationSearch, key, func() (any, error) { return searcher.SearchCards(ctx, request) })
	if err != nil {
		return card.SearchResult{}, err
	}
	return value.(card.SearchResult), nil
}

// GetRulings returns rulings for an Oracle card under the cache freshness policy.
func (c *Cache) GetRulings(ctx context.Context, selector card.OracleCardSelector) (card.RulingsResult, error) {
	key := rulingsKey(selector)
	value, err := c.get(ctx, operationRulings, key, func() (any, error) { return c.inner.GetRulings(ctx, selector) })
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
func searchKey(request card.SearchRequest) string {
	position := ""
	if request.Continuation != nil {
		position = request.Continuation.Position
	}
	encoded, _ := json.Marshal(struct {
		Query string                `json:"query"`
		Mode  card.SearchResultMode `json:"mode"`
		Size  int                   `json:"size"`
		Page  string                `json:"page"`
	}{strings.TrimSpace(request.Query), request.ResultMode, request.PageSize, position})
	return "v1:card-search:" + string(encoded)
}

func rulingsKey(s card.OracleCardSelector) string {
	if s.OracleID != "" {
		return "v1:rulings:oracle-id:" + strings.ToLower(strings.TrimSpace(s.OracleID))
	}
	return "v1:rulings:exact-name:" + strings.ToLower(strings.TrimSpace(s.Name))
}

func (c *Cache) get(ctx context.Context, operation operation, key string, live func() (any, error)) (any, error) {
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
		if cached, age, ok := c.read(operation, key); ok && age <= c.retainFor {
			cached = markStale(cached)
			f.value = cached
			return cached, nil
		}
	}
	f.err = err
	return nil, err
}

func markStale(value any) any {
	copy := reflect.New(reflect.TypeOf(value))
	copy.Elem().Set(reflect.ValueOf(value))
	copy.Elem().FieldByName("Provenance").FieldByName("Stale").SetBool(true)
	return copy.Elem().Interface()
}

func (c *Cache) read(operation operation, key string) (any, time.Duration, bool) {
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
	case operationOracle:
		value = &card.OracleCardResult{}
	case operationPrinting:
		value = &card.PrintingResult{}
	case operationLegality:
		value = &card.LegalityResult{}
	case operationSearch:
		value = &card.SearchResult{}
	case operationRulings:
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
	if age > c.retainFor {
		c.removeInvalid(path, "expired cache entry")
		return nil, age, false
	}
	record.LastAccess = now
	if updated, err := json.Marshal(record); err == nil {
		if err := c.fs.atomicWrite(path, updated); err != nil {
			c.diagnostic("cache last-access update failed for %s: %v", path, err)
		}
	}
	result := reflect.ValueOf(value).Elem()
	result.FieldByName("Provenance").FieldByName("Stale").SetBool(false)
	return result.Interface(), age, true
}

func (c *Cache) write(operation operation, key string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	fetched := reflect.ValueOf(value).FieldByName("Provenance").FieldByName("FetchedAt").Interface().(time.Time).UTC()
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
	defer func() { _ = os.Remove(name) }()
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
				return fmt.Errorf("account for unreadable cache entry %s: %w", path, errors.Join(e, fmt.Errorf("remove: %w", removeErr)))
			}
			continue
		}
		data, e := c.fs.readFile(path)
		if e != nil {
			if removeErr := c.fs.remove(path); removeErr != nil {
				return fmt.Errorf("account for unreadable cache entry %s (%d bytes): %w", path, info.Size(), errors.Join(e, fmt.Errorf("remove: %w", removeErr)))
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
