package scryfall_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/scryfall"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestTransientFailuresRetryAtMostTwice(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return response(http.StatusServiceUnavailable, `private upstream body`), nil
	})}
	_, err := scryfall.NewAdapterWithOptions("https://retry.scryfall.test", client, scryfall.Options{MaxRetries: 2}).GetOracleCard(context.Background(), card.OracleCardSelector{Name: "Example"})
	if !errors.Is(err, card.ErrUpstreamUnavailable) || calls != 3 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func TestDefinitiveClientFailuresAreNotRetried(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusBadRequest} {
		calls := 0
		client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return response(status, `do not expose me`), nil
		})}
		_, _ = scryfall.NewAdapterWithOptions(fmt.Sprintf("https://no-retry-%d.scryfall.test", status), client, scryfall.Options{MaxRetries: 2}).GetOracleCard(context.Background(), card.OracleCardSelector{Name: "Example"})
		if calls != 1 {
			t.Fatalf("status %d calls=%d", status, calls)
		}
	}
}

func TestRetryAfterIsHonored(t *testing.T) {
	calls := 0
	started := time.Now()
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			res := response(http.StatusTooManyRequests, `{}`)
			res.Header.Set("Retry-After", "1")
			return res, nil
		}
		return response(http.StatusOK, `{"oracle_id":"11111111-1111-4111-8111-111111111111","name":"Example","layout":"normal","colors":[],"color_identity":[],"keywords":[]}`), nil
	})}
	_, err := scryfall.NewAdapterWithOptions("https://retry-after.scryfall.test", client, scryfall.Options{MaxRetries: 1}).GetOracleCard(context.Background(), card.OracleCardSelector{Name: "Example"})
	if err != nil || time.Since(started) < time.Second {
		t.Fatalf("error=%v elapsed=%s", err, time.Since(started))
	}
}

func TestExhaustedRateLimitRemainsRateLimited(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		res := response(http.StatusTooManyRequests, `private body`)
		res.Header.Set("Retry-After", "0")
		return res, nil
	})}
	_, err := scryfall.NewAdapterWithOptions("https://rate-limit.scryfall.test", client, scryfall.Options{MaxRetries: 1}).GetOracleCard(context.Background(), card.OracleCardSelector{Name: "Example"})
	if !errors.Is(err, card.ErrRateLimited) {
		t.Fatalf("error=%v", err)
	}
}

func TestSharedLimiterSpacesNamedRequests(t *testing.T) {
	var mu sync.Mutex
	var calls []time.Time
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		calls = append(calls, time.Now())
		mu.Unlock()
		return response(http.StatusOK, `{"name":"Example","layout":"normal","colors":[],"color_identity":[],"keywords":[]}`), nil
	})}
	a := scryfall.NewAdapterWithOptions("https://spacing.scryfall.test", client, scryfall.Options{NamedInterval: 40 * time.Millisecond})
	b := scryfall.NewAdapterWithOptions("https://spacing.scryfall.test", client, scryfall.Options{NamedInterval: 40 * time.Millisecond})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = a.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "A"})
	}()
	go func() {
		defer wg.Done()
		_, _ = b.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "B"})
	}()
	wg.Wait()
	if len(calls) != 2 || calls[1].Sub(calls[0]) < 35*time.Millisecond {
		t.Fatalf("call times = %v", calls)
	}
}

func TestLimiterDeadlineReturnsRateLimited(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"name":"Example","layout":"normal","colors":[],"color_identity":[],"keywords":[]}`), nil
	})}
	a := scryfall.NewAdapterWithOptions("https://limited.scryfall.test", client, scryfall.Options{NamedInterval: time.Second})
	_, _ = a.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "A"})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := a.GetOracleCard(ctx, card.OracleCardSelector{Name: "B"})
	if !errors.Is(err, card.ErrRateLimited) {
		t.Fatalf("error=%v", err)
	}
}

func TestCancellationStopsLimiterWait(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"name":"Example","layout":"normal","colors":[],"color_identity":[],"keywords":[]}`), nil
	})}
	a := scryfall.NewAdapterWithOptions("https://cancel.scryfall.test", client, scryfall.Options{NamedInterval: time.Second})
	_, _ = a.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "A"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := a.GetOracleCard(ctx, card.OracleCardSelector{Name: "B"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}

func TestCrossHostRedirectIsRejectedBeforeHeadersAreForwarded(t *testing.T) {
	forwarded := false
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "evil.test" {
			forwarded = r.Header.Get("User-Agent") != "" || r.Header.Get("Accept") != ""
			return response(http.StatusOK, `{}`), nil
		}
		res := response(http.StatusFound, ``)
		res.Header.Set("Location", "https://evil.test/cards/named")
		return res, nil
	})}
	_, err := scryfall.NewAdapterWithOptions("https://boundary.scryfall.test", client, scryfall.Options{}).GetOracleCard(context.Background(), card.OracleCardSelector{Name: "Example"})
	if !errors.Is(err, card.ErrUpstreamUnavailable) || forwarded {
		t.Fatalf("error=%v forwarded=%v", err, forwarded)
	}
}

func TestExactNameLookupTranslatesScryfallCard(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/cards/named" || r.URL.Query().Get("exact") != "Delver of Secrets" {
			t.Errorf("request URL = %s", r.URL)
		}
		if r.Header.Get("User-Agent") == "" || r.Header.Get("Accept") != "application/json" {
			t.Errorf("headers = %#v", r.Header)
		}
		return response(http.StatusOK, `{
          "id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","oracle_id":"11111111-1111-4111-8111-111111111111",
          "name":"Delver of Secrets // Insectile Aberration","layout":"transform","cmc":1,
          "colors":["U"],"color_identity":["U"],"keywords":["Transform"],
          "card_faces":[
            {"name":"Delver of Secrets","mana_cost":"{U}","type_line":"Creature — Human Wizard","oracle_text":"At the beginning of your upkeep..."},
            {"name":"Insectile Aberration","type_line":"Creature — Human Insect","oracle_text":"Flying"}
          ]
        }`), nil
	})}

	result, err := scryfall.NewAdapter("https://api.scryfall.test", client).GetOracleCard(context.Background(), card.OracleCardSelector{Name: "Delver of Secrets"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Card.OracleID != "11111111-1111-4111-8111-111111111111" || len(result.Card.Faces) != 2 || result.Card.Faces[1].Name != "Insectile Aberration" {
		t.Fatalf("card = %#v", result.Card)
	}
	if result.SourceURL == "" || result.FetchedAt.IsZero() || result.Stale {
		t.Fatalf("provenance = %#v", result)
	}
}

func TestOracleIDLookupUsesExactOracleQuery(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/cards/search" || r.URL.Query().Get("q") != "oracleid:"+id || r.URL.Query().Get("unique") != "cards" {
			t.Errorf("request URL = %s", r.URL)
		}
		return response(http.StatusOK, `{"data":[{"oracle_id":"`+id+`","name":"Example","layout":"normal","cmc":2,"colors":[],"color_identity":[],"keywords":[],"mana_cost":"{2}","type_line":"Artifact","oracle_text":"Text"}]}`), nil
	})}
	result, err := scryfall.NewAdapter("https://api.scryfall.test", client).GetOracleCard(context.Background(), card.OracleCardSelector{OracleID: id})
	if err != nil {
		t.Fatal(err)
	}
	if result.Card.OracleID != id || result.Card.Name != "Example" {
		t.Fatalf("card = %#v", result.Card)
	}
}

func TestScryfallNotFoundIsDomainNotFound(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { return response(http.StatusNotFound, `{}`), nil })}
	_, err := scryfall.NewAdapter("https://api.scryfall.test", client).GetOracleCard(context.Background(), card.OracleCardSelector{Name: "Missing"})
	if err != card.ErrNotFound {
		t.Fatalf("error = %v", err)
	}
}
