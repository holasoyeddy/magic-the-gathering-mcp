package scryfall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
)

const userAgent = "magic-the-gathering-mcp/0.1 (+https://github.com/holasoyeddy/magic-the-gathering-mcp)"

type Options struct {
	NamedInterval time.Duration
	MaxRetries    int
}
type Adapter struct {
	baseURL    *url.URL
	client     *http.Client
	now        func() time.Time
	interval   time.Duration
	maxRetries int
}

func NewAdapter(baseURL string, client *http.Client) *Adapter {
	return NewAdapterWithOptions(baseURL, client, Options{NamedInterval: 500 * time.Millisecond, MaxRetries: 2})
}
func NewAdapterWithOptions(baseURL string, client *http.Client, options Options) *Adapter {
	base, _ := url.Parse(strings.TrimRight(baseURL, "/"))
	if client == nil {
		client = http.DefaultClient
	}
	copyClient := *client
	originalRedirect := copyClient.CheckRedirect
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if base == nil || !sameDestination(req.URL, base) {
			req.Header.Del("User-Agent")
			req.Header.Del("Accept")
			return errors.New("redirect outside configured Scryfall host rejected")
		}
		if originalRedirect != nil {
			return originalRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &Adapter{baseURL: base, client: &copyClient, now: time.Now, interval: options.NamedInterval, maxRetries: options.MaxRetries}
}

type upstreamCard struct {
	OracleID      string         `json:"oracle_id"`
	Name          string         `json:"name"`
	Layout        string         `json:"layout"`
	ManaCost      *string        `json:"mana_cost"`
	CMC           float64        `json:"cmc"`
	TypeLine      *string        `json:"type_line"`
	OracleText    *string        `json:"oracle_text"`
	Colors        []string       `json:"colors"`
	ColorIdentity []string       `json:"color_identity"`
	Keywords      []string       `json:"keywords"`
	ProducedMana  []string       `json:"produced_mana"`
	CardFaces     []upstreamFace `json:"card_faces"`
}
type upstreamFace struct {
	Name       string  `json:"name"`
	ManaCost   *string `json:"mana_cost"`
	TypeLine   *string `json:"type_line"`
	OracleText *string `json:"oracle_text"`
}

func (a *Adapter) GetOracleCard(ctx context.Context, selector card.OracleCardSelector) (card.OracleCardResult, error) {
	if a.baseURL == nil || a.baseURL.Scheme != "https" || a.baseURL.Host == "" {
		return card.OracleCardResult{}, card.ErrUpstreamUnavailable
	}
	endpoint := *a.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/cards/named"
	endpoint.RawQuery = url.Values{"exact": {selector.Name}}.Encode()
	isSearch := selector.OracleID != ""
	if isSearch {
		endpoint.Path = strings.TrimRight(a.baseURL.Path, "/") + "/cards/search"
		endpoint.RawQuery = url.Values{"q": {"oracleid:" + selector.OracleID}, "unique": {"cards"}}.Encode()
	}
	var exhaustedError = card.ErrUpstreamUnavailable
	for attempt := 0; attempt <= a.maxRetries; attempt++ {
		if err := sharedLimiters.wait(ctx, a.baseURL.Host, a.interval); err != nil {
			if errors.Is(err, context.Canceled) {
				return card.OracleCardResult{}, err
			}
			return card.OracleCardResult{}, card.ErrRateLimited
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return card.OracleCardResult{}, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", userAgent)
		response, err := a.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return card.OracleCardResult{}, ctx.Err()
			}
			continue
		}
		result, retry, retryAfter, err := a.consume(response, endpoint.String(), isSearch)
		if !retry {
			return result, err
		}
		if errors.Is(err, card.ErrRateLimited) {
			exhaustedError = card.ErrRateLimited
		}
		if attempt < a.maxRetries && retryAfter > 0 {
			timer := time.NewTimer(retryAfter)
			select {
			case <-ctx.Done():
				timer.Stop()
				return card.OracleCardResult{}, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return card.OracleCardResult{}, exhaustedError
}

func (a *Adapter) consume(response *http.Response, endpoint string, isSearch bool) (card.OracleCardResult, bool, time.Duration, error) {
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		io.Copy(io.Discard, response.Body)
		return card.OracleCardResult{}, false, 0, card.ErrNotFound
	}
	if response.StatusCode == http.StatusTooManyRequests {
		io.Copy(io.Discard, response.Body)
		return card.OracleCardResult{}, true, parseRetryAfter(response.Header.Get("Retry-After"), a.now()), card.ErrRateLimited
	}
	if response.StatusCode >= 500 {
		io.Copy(io.Discard, response.Body)
		return card.OracleCardResult{}, true, 0, card.ErrUpstreamUnavailable
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		io.Copy(io.Discard, response.Body)
		return card.OracleCardResult{}, false, 0, card.ErrInvalidIdentifier
	}
	var raw upstreamCard
	if isSearch {
		var page struct {
			Data []upstreamCard `json:"data"`
		}
		if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
			return card.OracleCardResult{}, false, 0, fmt.Errorf("decode Scryfall response: %w", err)
		}
		if len(page.Data) != 1 {
			return card.OracleCardResult{}, false, 0, card.ErrNotFound
		}
		raw = page.Data[0]
	} else if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return card.OracleCardResult{}, false, 0, fmt.Errorf("decode Scryfall response: %w", err)
	}
	return card.OracleCardResult{Card: translate(raw), SourceURL: endpoint, FetchedAt: a.now().UTC(), Stale: false}, false, 0, nil
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil && when.After(now) {
		return when.Sub(now)
	}
	return 0
}
func sameDestination(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

type limiterRegistry struct {
	mu   sync.Mutex
	next map[string]time.Time
}

var sharedLimiters = limiterRegistry{next: make(map[string]time.Time)}

func (r *limiterRegistry) wait(ctx context.Context, host string, interval time.Duration) error {
	if interval <= 0 {
		return nil
	}
	r.mu.Lock()
	now := time.Now()
	ready := r.next[host]
	if ready.Before(now) {
		ready = now
	}
	r.next[host] = ready.Add(interval)
	r.mu.Unlock()
	timer := time.NewTimer(time.Until(ready))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func translate(raw upstreamCard) card.OracleCard {
	faces := make([]card.OracleCardFace, 0, len(raw.CardFaces))
	for _, face := range raw.CardFaces {
		faces = append(faces, card.OracleCardFace{Name: face.Name, ManaCost: face.ManaCost, TypeLine: face.TypeLine, OracleText: face.OracleText})
	}
	return card.OracleCard{OracleID: raw.OracleID, Name: raw.Name, Layout: raw.Layout, Faces: faces, ManaCost: raw.ManaCost, ManaValue: raw.CMC, TypeLine: raw.TypeLine, OracleText: raw.OracleText, Colors: nonNil(raw.Colors), ColorIdentity: nonNil(raw.ColorIdentity), Keywords: nonNil(raw.Keywords), ProducedMana: raw.ProducedMana}
}
func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
