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

const defaultRequestTimeout = 10 * time.Second

type endpointClass string

const (
	classNamedSearch endpointClass = "named-search"
	classOther       endpointClass = "other"
)

type Options struct {
	NamedInterval  time.Duration
	OtherInterval  time.Duration
	MaxRetries     int
	RequestTimeout time.Duration
}
type Adapter struct {
	baseURL       *url.URL
	client        *http.Client
	now           func() time.Time
	namedInterval time.Duration
	otherInterval time.Duration
	maxRetries    int
}

func NewAdapter(baseURL string, client *http.Client) *Adapter {
	return NewAdapterWithOptions(baseURL, client, Options{NamedInterval: 500 * time.Millisecond, OtherInterval: 100 * time.Millisecond, MaxRetries: 2})
}
func NewAdapterWithOptions(baseURL string, client *http.Client, options Options) *Adapter {
	base, _ := url.Parse(strings.TrimRight(baseURL, "/"))
	if client == nil {
		client = http.DefaultClient
	}
	copyClient := *client
	requestTimeout := options.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = defaultRequestTimeout
	}
	if copyClient.Timeout <= 0 {
		copyClient.Timeout = requestTimeout
	}
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
	return &Adapter{baseURL: base, client: &copyClient, now: time.Now, namedInterval: options.NamedInterval, otherInterval: options.OtherInterval, maxRetries: options.MaxRetries}
}

type upstreamCard struct {
	ID              string            `json:"id"`
	OracleID        string            `json:"oracle_id"`
	Name            string            `json:"name"`
	Layout          string            `json:"layout"`
	ManaCost        *string           `json:"mana_cost"`
	CMC             float64           `json:"cmc"`
	TypeLine        *string           `json:"type_line"`
	OracleText      *string           `json:"oracle_text"`
	Colors          []string          `json:"colors"`
	ColorIdentity   []string          `json:"color_identity"`
	Keywords        []string          `json:"keywords"`
	ProducedMana    []string          `json:"produced_mana"`
	CardFaces       []upstreamFace    `json:"card_faces"`
	Set             string            `json:"set"`
	CollectorNumber string            `json:"collector_number"`
	Lang            string            `json:"lang"`
	Legalities      map[string]string `json:"legalities"`
}

func (a *Adapter) SearchCards(ctx context.Context, request card.SearchRequest) (card.SearchResult, error) {
	if a.baseURL == nil || a.baseURL.Scheme != "https" || a.baseURL.Host == "" {
		return card.SearchResult{}, card.ErrUpstreamUnavailable
	}
	unique := "cards"
	if request.ResultMode == card.SearchPrintings {
		unique = "prints"
	}
	endpoint := *a.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/cards/search"
	endpoint.RawQuery = url.Values{"q": {request.Query}, "unique": {unique}, "page": {"1"}}.Encode()
	offset := 0
	if request.Continuation != nil {
		position := request.Continuation.Position
		if separator := strings.LastIndex(position, "#offset="); separator >= 0 {
			parsed, err := strconv.Atoi(position[separator+8:])
			if err != nil || parsed < 0 {
				return card.SearchResult{}, card.ErrInvalidIdentifier
			}
			offset = parsed
			position = position[:separator]
		}
		next, err := url.Parse(position)
		if err != nil || !sameDestination(next, a.baseURL) || next.Path != endpoint.Path || next.Query().Get("q") != request.Query || next.Query().Get("unique") != unique || next.Query().Get("page") == "" {
			return card.SearchResult{}, card.ErrInvalidIdentifier
		}
		endpoint = *next
	}
	exhausted := error(card.ErrUpstreamUnavailable)
	for attempt := 0; attempt <= a.maxRetries; attempt++ {
		if err := sharedLimiters.wait(ctx, a.baseURL.Host, classNamedSearch, a.namedInterval); err != nil {
			if errors.Is(err, context.Canceled) {
				return card.SearchResult{}, err
			}
			return card.SearchResult{}, card.ErrRateLimited
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return card.SearchResult{}, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", userAgent)
		response, err := a.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return card.SearchResult{}, ctx.Err()
			}
			continue
		}
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			retryAfter := parseRetryAfter(response.Header.Get("Retry-After"), a.now())
			response.Body.Close()
			if response.StatusCode == http.StatusTooManyRequests {
				exhausted = card.ErrRateLimited
			}
			if attempt < a.maxRetries && retryAfter > 0 {
				timer := time.NewTimer(retryAfter)
				select {
				case <-ctx.Done():
					timer.Stop()
					return card.SearchResult{}, ctx.Err()
				case <-timer.C:
				}
			}
			continue
		}
		if response.StatusCode == http.StatusBadRequest {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			return card.SearchResult{}, card.ErrInvalidQuery
		}
		if response.StatusCode == http.StatusNotFound {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			return card.SearchResult{Results: []card.SearchSummary{}, SourceURL: endpoint.String(), FetchedAt: a.now().UTC()}, nil
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			return card.SearchResult{}, card.ErrUpstreamUnavailable
		}
		var page struct {
			Data     []upstreamCard `json:"data"`
			HasMore  bool           `json:"has_more"`
			NextPage string         `json:"next_page"`
		}
		err = json.NewDecoder(response.Body).Decode(&page)
		response.Body.Close()
		if err != nil {
			return card.SearchResult{}, fmt.Errorf("decode Scryfall search response: %w", err)
		}
		if offset > len(page.Data) {
			return card.SearchResult{}, card.ErrInvalidIdentifier
		}
		end := offset + request.PageSize
		if end > len(page.Data) {
			end = len(page.Data)
		}
		summaries := make([]card.SearchSummary, 0, end-offset)
		for _, raw := range page.Data[offset:end] {
			summaries = append(summaries, searchSummary(raw, request.ResultMode))
		}
		result := card.SearchResult{Results: summaries, SourceURL: endpoint.String(), FetchedAt: a.now().UTC()}
		if end < len(page.Data) {
			result.Next = &card.SearchContinuation{Position: endpoint.String() + "#offset=" + strconv.Itoa(end)}
		} else if page.HasMore && page.NextPage != "" {
			result.Next = &card.SearchContinuation{Position: page.NextPage}
		}
		return result, nil
	}
	return card.SearchResult{}, exhausted
}

func searchSummary(raw upstreamCard, mode card.SearchResultMode) card.SearchSummary {
	mana, typeLine := "", ""
	if raw.ManaCost != nil {
		mana = *raw.ManaCost
	}
	if raw.TypeLine != nil {
		typeLine = *raw.TypeLine
	}
	result := card.SearchSummary{OracleID: raw.OracleID, Name: raw.Name, ManaCost: mana, ManaValue: raw.CMC, TypeLine: typeLine}
	if mode == card.SearchPrintings {
		result.ScryfallID = raw.ID
		result.SetCode = strings.ToLower(raw.Set)
		result.CollectorNumber = raw.CollectorNumber
		result.Language = raw.Lang
	}
	return result
}

type upstreamRuling struct {
	PublishedAt string `json:"published_at"`
	Comment     string `json:"comment"`
}
type upstreamFace struct {
	Name        string            `json:"name"`
	PrintedName *string           `json:"printed_name"`
	ManaCost    *string           `json:"mana_cost"`
	TypeLine    *string           `json:"type_line"`
	OracleText  *string           `json:"oracle_text"`
	PrintedText *string           `json:"printed_text"`
	ImageURIs   map[string]string `json:"image_uris"`
}

type upstreamPrinting struct {
	ID              string            `json:"id"`
	OracleID        *string           `json:"oracle_id"`
	Name            string            `json:"name"`
	Lang            string            `json:"lang"`
	PrintedName     *string           `json:"printed_name"`
	PrintedTypeLine *string           `json:"printed_type_line"`
	PrintedText     *string           `json:"printed_text"`
	Set             string            `json:"set"`
	SetName         string            `json:"set_name"`
	CollectorNumber string            `json:"collector_number"`
	Rarity          string            `json:"rarity"`
	ReleasedAt      string            `json:"released_at"`
	Artist          *string           `json:"artist"`
	Finishes        []string          `json:"finishes"`
	Frame           string            `json:"frame"`
	FrameEffects    []string          `json:"frame_effects"`
	BorderColor     string            `json:"border_color"`
	FullArt         bool              `json:"full_art"`
	Textless        bool              `json:"textless"`
	ImageURIs       map[string]string `json:"image_uris"`
	CardFaces       []upstreamFace    `json:"card_faces"`
}

func (a *Adapter) GetOracleCard(ctx context.Context, selector card.OracleCardSelector) (card.OracleCardResult, error) {
	result, err := a.getExactCard(ctx, selector)
	if err != nil {
		return card.OracleCardResult{}, err
	}
	return card.OracleCardResult{Card: translate(result.card), SourceURL: result.sourceURL, FetchedAt: result.fetchedAt}, nil
}

func (a *Adapter) GetCardLegality(ctx context.Context, selector card.OracleCardSelector) (card.LegalityResult, error) {
	result, err := a.getExactCard(ctx, selector)
	if err != nil {
		return card.LegalityResult{}, err
	}
	legalities, err := translateLegalities(result.card.Legalities)
	if err != nil {
		return card.LegalityResult{}, err
	}
	return card.LegalityResult{
		Legality:  card.CardLegality{OracleID: result.card.OracleID, Name: result.card.Name, Formats: legalities},
		SourceURL: result.sourceURL, FetchedAt: result.fetchedAt,
	}, nil
}

type exactCardResult struct {
	card      upstreamCard
	sourceURL string
	fetchedAt time.Time
}

func (a *Adapter) getExactCard(ctx context.Context, selector card.OracleCardSelector) (exactCardResult, error) {
	if a.baseURL == nil || a.baseURL.Scheme != "https" || a.baseURL.Host == "" {
		return exactCardResult{}, card.ErrUpstreamUnavailable
	}
	endpoint := *a.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/cards/named"
	endpoint.RawQuery = url.Values{"exact": {selector.Name}}.Encode()
	isSearch := selector.OracleID != ""
	if isSearch {
		endpoint.Path = strings.TrimRight(a.baseURL.Path, "/") + "/cards/search"
		endpoint.RawQuery = url.Values{"q": {"oracleid:" + selector.OracleID}, "unique": {"cards"}}.Encode()
	}
	exhaustedError := card.ErrUpstreamUnavailable
	for attempt := 0; attempt <= a.maxRetries; attempt++ {
		if err := sharedLimiters.wait(ctx, a.baseURL.Host, classNamedSearch, a.namedInterval); err != nil {
			if errors.Is(err, context.Canceled) {
				return exactCardResult{}, err
			}
			return exactCardResult{}, card.ErrRateLimited
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return exactCardResult{}, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", userAgent)
		response, err := a.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return exactCardResult{}, ctx.Err()
			}
			continue
		}
		raw, retry, retryAfter, err := a.consumeExactCard(response, isSearch)
		if !retry {
			if err != nil {
				return exactCardResult{}, err
			}
			return exactCardResult{card: raw, sourceURL: endpoint.String(), fetchedAt: a.now().UTC()}, nil
		}
		if errors.Is(err, card.ErrRateLimited) {
			exhaustedError = card.ErrRateLimited
		}
		if attempt < a.maxRetries && retryAfter > 0 {
			timer := time.NewTimer(retryAfter)
			select {
			case <-ctx.Done():
				timer.Stop()
				return exactCardResult{}, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return exactCardResult{}, exhaustedError
}

func (a *Adapter) consumeExactCard(response *http.Response, isSearch bool) (upstreamCard, bool, time.Duration, error) {
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		io.Copy(io.Discard, response.Body)
		return upstreamCard{}, false, 0, card.ErrNotFound
	}
	if response.StatusCode == http.StatusTooManyRequests {
		io.Copy(io.Discard, response.Body)
		return upstreamCard{}, true, parseRetryAfter(response.Header.Get("Retry-After"), a.now()), card.ErrRateLimited
	}
	if response.StatusCode >= 500 {
		io.Copy(io.Discard, response.Body)
		return upstreamCard{}, true, 0, card.ErrUpstreamUnavailable
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		io.Copy(io.Discard, response.Body)
		return upstreamCard{}, false, 0, card.ErrInvalidIdentifier
	}
	var raw upstreamCard
	if isSearch {
		var page struct {
			Data []upstreamCard `json:"data"`
		}
		if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
			return upstreamCard{}, false, 0, fmt.Errorf("decode Scryfall response: %w", err)
		}
		if len(page.Data) != 1 {
			return upstreamCard{}, false, 0, card.ErrNotFound
		}
		raw = page.Data[0]
	} else if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return upstreamCard{}, false, 0, fmt.Errorf("decode Scryfall response: %w", err)
	}
	return raw, false, 0, nil
}

func translateLegalities(raw map[string]string) (map[string]card.LegalityStatus, error) {
	translated := make(map[string]card.LegalityStatus, len(raw))
	for format, rawStatus := range raw {
		status := card.LegalityStatus(rawStatus)
		switch status {
		case card.LegalityLegal, card.LegalityNotLegal, card.LegalityRestricted, card.LegalityBanned:
			translated[format] = status
		default:
			return nil, errors.New("invalid legality status from Scryfall")
		}
	}
	return translated, nil
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
	mu     sync.Mutex
	states map[limiterKey]*limiterState
}

type limiterKey struct {
	host  string
	class endpointClass
}
type limiterWaiter struct{ wake chan struct{} }
type limiterState struct {
	last  time.Time
	queue []*limiterWaiter
}

var sharedLimiters = limiterRegistry{states: make(map[limiterKey]*limiterState)}

func (r *limiterRegistry) wait(ctx context.Context, host string, class endpointClass, interval time.Duration) error {
	if interval <= 0 {
		return nil
	}
	key := limiterKey{host: host, class: class}
	w := &limiterWaiter{wake: make(chan struct{}, 1)}
	r.mu.Lock()
	state := r.states[key]
	if state == nil {
		state = &limiterState{}
		r.states[key] = state
	}
	state.queue = append(state.queue, w)
	if len(state.queue) == 1 {
		w.wake <- struct{}{}
	}
	r.mu.Unlock()
	for {
		select {
		case <-ctx.Done():
			r.cancel(key, w)
			return ctx.Err()
		case <-w.wake:
			r.mu.Lock()
			state = r.states[key]
			if state == nil || len(state.queue) == 0 || state.queue[0] != w {
				r.mu.Unlock()
				continue
			}
			ready := state.last.Add(interval)
			r.mu.Unlock()
			delay := time.Until(ready)
			if delay > 0 {
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					r.cancel(key, w)
					return ctx.Err()
				case <-timer.C:
				}
			}
			if err := ctx.Err(); err != nil {
				r.cancel(key, w)
				return err
			}
			r.mu.Lock()
			state = r.states[key]
			if state != nil && len(state.queue) > 0 && state.queue[0] == w {
				state.queue = state.queue[1:]
				state.last = time.Now()
				if len(state.queue) > 0 {
					state.queue[0].wake <- struct{}{}
				}
				r.mu.Unlock()
				return nil
			}
			r.mu.Unlock()
		}
	}
}

func (r *limiterRegistry) cancel(key limiterKey, target *limiterWaiter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.states[key]
	if state == nil {
		return
	}
	for i, waiter := range state.queue {
		if waiter != target {
			continue
		}
		wasHead := i == 0
		state.queue = append(state.queue[:i], state.queue[i+1:]...)
		if wasHead && len(state.queue) > 0 {
			state.queue[0].wake <- struct{}{}
		}
		return
	}
}

func (a *Adapter) GetPrinting(ctx context.Context, selector card.PrintingSelector) (card.PrintingResult, error) {
	endpointURL := *a.baseURL
	endpointURL.Path = strings.TrimRight(endpointURL.Path, "/") + "/cards/" + url.PathEscape(selector.ScryfallID)
	if selector.ScryfallID == "" {
		endpointURL.Path = strings.TrimRight(a.baseURL.Path, "/") + "/cards/" + url.PathEscape(strings.ToLower(selector.SetCode)) + "/" + url.PathEscape(selector.CollectorNumber)
	}
	endpoint := endpointURL.String()
	exhaustedError := card.ErrUpstreamUnavailable
	for attempt := 0; attempt <= a.maxRetries; attempt++ {
		if err := sharedLimiters.wait(ctx, a.baseURL.Host, classOther, a.otherInterval); err != nil {
			if errors.Is(err, context.Canceled) {
				return card.PrintingResult{}, err
			}
			return card.PrintingResult{}, card.ErrRateLimited
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return card.PrintingResult{}, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", userAgent)
		response, err := a.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return card.PrintingResult{}, ctx.Err()
			}
			continue
		}
		if response.StatusCode == http.StatusNotFound {
			response.Body.Close()
			return card.PrintingResult{}, card.ErrNotFound
		}
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			retryAfter := parseRetryAfter(response.Header.Get("Retry-After"), a.now())
			response.Body.Close()
			if response.StatusCode == http.StatusTooManyRequests {
				exhaustedError = card.ErrRateLimited
			}
			if attempt < a.maxRetries && retryAfter > 0 {
				timer := time.NewTimer(retryAfter)
				select {
				case <-ctx.Done():
					timer.Stop()
					return card.PrintingResult{}, ctx.Err()
				case <-timer.C:
				}
			}
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			response.Body.Close()
			return card.PrintingResult{}, card.ErrInvalidIdentifier
		}
		var raw upstreamPrinting
		decodeErr := json.NewDecoder(response.Body).Decode(&raw)
		response.Body.Close()
		if decodeErr != nil {
			return card.PrintingResult{}, fmt.Errorf("decode Scryfall response: %w", decodeErr)
		}
		return card.PrintingResult{Printing: translatePrinting(raw), SourceURL: endpoint, FetchedAt: a.now().UTC(), Stale: false}, nil
	}
	return card.PrintingResult{}, exhaustedError
}

func (a *Adapter) GetRulings(ctx context.Context, selector card.OracleCardSelector) (card.RulingsResult, error) {
	if a.baseURL == nil || a.baseURL.Scheme != "https" || a.baseURL.Host == "" {
		return card.RulingsResult{}, card.ErrUpstreamUnavailable
	}
	lookup := *a.baseURL
	lookup.Path = strings.TrimRight(lookup.Path, "/") + "/cards/named"
	lookup.RawQuery = url.Values{"exact": {selector.Name}}.Encode()
	isSearch := selector.OracleID != ""
	if isSearch {
		lookup.Path = strings.TrimRight(a.baseURL.Path, "/") + "/cards/search"
		lookup.RawQuery = url.Values{"q": {"oracleid:" + selector.OracleID}, "unique": {"cards"}}.Encode()
	}
	var page struct {
		Data []upstreamCard `json:"data"`
	}
	var single upstreamCard
	if isSearch {
		if err := a.getJSON(ctx, lookup.String(), classNamedSearch, a.namedInterval, &page); err != nil {
			return card.RulingsResult{}, err
		}
		if len(page.Data) != 1 {
			return card.RulingsResult{}, card.ErrNotFound
		}
		single = page.Data[0]
	} else if err := a.getJSON(ctx, lookup.String(), classNamedSearch, a.namedInterval, &single); err != nil {
		return card.RulingsResult{}, err
	}
	if single.ID == "" {
		return card.RulingsResult{}, card.ErrUpstreamUnavailable
	}
	endpoint := *a.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/cards/" + url.PathEscape(single.ID) + "/rulings"
	endpoint.RawQuery = ""
	var rulingsPage struct {
		Data []upstreamRuling `json:"data"`
	}
	if err := a.getJSON(ctx, endpoint.String(), classOther, a.otherInterval, &rulingsPage); err != nil {
		return card.RulingsResult{}, err
	}
	rulings := make([]card.Ruling, 0, len(rulingsPage.Data))
	for _, ruling := range rulingsPage.Data {
		rulings = append(rulings, card.Ruling{PublishedAt: ruling.PublishedAt, Comment: ruling.Comment})
	}
	return card.RulingsResult{Rulings: rulings, SourceURL: endpoint.String(), FetchedAt: a.now().UTC()}, nil
}

func (a *Adapter) getJSON(ctx context.Context, endpoint string, class endpointClass, interval time.Duration, target any) error {
	exhausted := card.ErrUpstreamUnavailable
	for attempt := 0; attempt <= a.maxRetries; attempt++ {
		if err := sharedLimiters.wait(ctx, a.baseURL.Host, class, interval); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			return card.ErrRateLimited
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", userAgent)
		response, err := a.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}
		if response.StatusCode == http.StatusNotFound {
			response.Body.Close()
			return card.ErrNotFound
		}
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			retryAfter := parseRetryAfter(response.Header.Get("Retry-After"), a.now())
			response.Body.Close()
			if response.StatusCode == http.StatusTooManyRequests {
				exhausted = card.ErrRateLimited
			}
			if attempt < a.maxRetries && retryAfter > 0 {
				timer := time.NewTimer(retryAfter)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
			}
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			response.Body.Close()
			return card.ErrInvalidIdentifier
		}
		err = json.NewDecoder(response.Body).Decode(target)
		response.Body.Close()
		if err != nil {
			return fmt.Errorf("decode Scryfall response: %w", err)
		}
		return nil
	}
	return exhausted
}

func translatePrinting(raw upstreamPrinting) card.Printing {
	faces := make([]card.PrintingFace, 0, len(raw.CardFaces))
	for _, face := range raw.CardFaces {
		faces = append(faces, card.PrintingFace{Name: face.Name, PrintedName: face.PrintedName, ManaCost: face.ManaCost, TypeLine: face.TypeLine, PrintedText: face.PrintedText, ImageURLs: scryfallImages(face.ImageURIs)})
	}
	return card.Printing{
		ScryfallID: raw.ID, OracleID: raw.OracleID, Name: raw.Name, Language: raw.Lang,
		PrintedName: raw.PrintedName, PrintedTypeLine: raw.PrintedTypeLine, PrintedText: raw.PrintedText,
		SetCode: strings.ToLower(raw.Set), SetName: raw.SetName, CollectorNumber: raw.CollectorNumber,
		Rarity: raw.Rarity, ReleasedAt: raw.ReleasedAt, Artist: raw.Artist, Finishes: nonNil(raw.Finishes),
		Frame: raw.Frame, FrameEffects: raw.FrameEffects, BorderColor: raw.BorderColor, FullArt: raw.FullArt, Textless: raw.Textless,
		ImageURLs: scryfallImages(raw.ImageURIs), Faces: faces,
	}
}

func scryfallImages(images map[string]string) map[string]string {
	var filtered map[string]string
	for size, value := range images {
		parsed, err := url.Parse(value)
		if err == nil && parsed.Scheme == "https" && parsed.Hostname() == "cards.scryfall.io" {
			if filtered == nil {
				filtered = make(map[string]string)
			}
			filtered[size] = value
		}
	}
	return filtered
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
