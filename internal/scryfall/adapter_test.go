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

func TestLimiterUsesEndpointClassIntervals(t *testing.T) {
	var mu sync.Mutex
	times := map[string][]time.Time{"named": {}, "other": {}}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		class := "other"
		body := `{"id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","name":"Print","lang":"en","set":"tst","collector_number":"1","finishes":[]}`
		if r.URL.Path == "/cards/named" {
			class = "named"
			body = `{"name":"Oracle","layout":"normal","colors":[],"color_identity":[],"keywords":[]}`
		}
		mu.Lock()
		times[class] = append(times[class], time.Now())
		mu.Unlock()
		return response(http.StatusOK, body), nil
	})}
	a := scryfall.NewAdapterWithOptions("https://classes.scryfall.test", client, scryfall.Options{NamedInterval: 60 * time.Millisecond, OtherInterval: 15 * time.Millisecond})
	var wg sync.WaitGroup
	wg.Add(4)
	for _, name := range []string{"A", "B"} {
		go func() {
			defer wg.Done()
			_, _ = a.GetOracleCard(context.Background(), card.OracleCardSelector{Name: name})
		}()
	}
	for _, id := range []string{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"} {
		go func() {
			defer wg.Done()
			_, _ = a.GetPrinting(context.Background(), card.PrintingSelector{ScryfallID: id})
		}()
	}
	wg.Wait()
	if got := times["named"]; len(got) != 2 || got[1].Sub(got[0]) < 55*time.Millisecond {
		t.Fatalf("named call times = %v", got)
	}
	if got := times["other"]; len(got) != 2 || got[1].Sub(got[0]) < 12*time.Millisecond || got[1].Sub(got[0]) >= 50*time.Millisecond {
		t.Fatalf("other call times = %v", got)
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

func TestCanceledLimiterWaitDoesNotReserveAnExtraInterval(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"name":"Example","layout":"normal","colors":[],"color_identity":[],"keywords":[]}`), nil
	})}
	interval := 80 * time.Millisecond
	a := scryfall.NewAdapterWithOptions("https://reclaim.scryfall.test", client, scryfall.Options{NamedInterval: interval})
	_, _ = a.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "A"})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := a.GetOracleCard(ctx, card.OracleCardSelector{Name: "B"}); done <- err }()
	time.Sleep(10 * time.Millisecond)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	started := time.Now()
	if _, err := a.GetOracleCard(context.Background(), card.OracleCardSelector{Name: "C"}); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed >= 130*time.Millisecond {
		t.Fatalf("later request delayed by canceled reservation: %s", elapsed)
	}
}

func TestHTTPAttemptIsBoundedWithoutCallerDeadline(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	started := time.Now()
	_, err := scryfall.NewAdapterWithOptions("https://timeout.scryfall.test", client, scryfall.Options{RequestTimeout: 20 * time.Millisecond}).GetOracleCard(context.Background(), card.OracleCardSelector{Name: "Example"})
	if !errors.Is(err, card.ErrUpstreamUnavailable) || time.Since(started) > 200*time.Millisecond {
		t.Fatalf("error=%v elapsed=%s", err, time.Since(started))
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

func TestCardLegalityTranslatesOnlyIdentityAndCompleteStatusMap(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/cards/named" || r.URL.Query().Get("exact") != "Example" {
			t.Errorf("request URL = %s", r.URL)
		}
		return response(http.StatusOK, `{
          "oracle_id":"11111111-1111-4111-8111-111111111111","name":"Example",
          "legalities":{"standard":"legal","future":"not_legal","vintage":"restricted","legacy":"banned"},
          "prices":{"usd":"999.99"},"oracle_text":"must not leak into legality"
        }`), nil
	})}
	result, err := scryfall.NewAdapter("https://api.scryfall.test", client).GetCardLegality(context.Background(), card.OracleCardSelector{Name: "Example"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Legality.OracleID != "11111111-1111-4111-8111-111111111111" || result.Legality.Name != "Example" || len(result.Legality.Formats) != 4 {
		t.Fatalf("legality = %#v", result.Legality)
	}
	if result.Legality.Formats["standard"] != card.LegalityLegal || result.Legality.Formats["future"] != card.LegalityNotLegal || result.Legality.Formats["vintage"] != card.LegalityRestricted || result.Legality.Formats["legacy"] != card.LegalityBanned {
		t.Fatalf("formats = %#v", result.Legality.Formats)
	}
	if result.SourceURL == "" || result.FetchedAt.IsZero() || result.Stale {
		t.Fatalf("provenance = %#v", result)
	}
}

func TestCardLegalityByOracleIDUsesExactOracleQuery(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/cards/search" || r.URL.Query().Get("q") != "oracleid:"+id || r.URL.Query().Get("unique") != "cards" {
			t.Errorf("request URL = %s", r.URL)
		}
		return response(http.StatusOK, `{"data":[{"oracle_id":"`+id+`","name":"Example","legalities":{"commander":"legal"}}]}`), nil
	})}
	result, err := scryfall.NewAdapter("https://api.scryfall.test", client).GetCardLegality(context.Background(), card.OracleCardSelector{OracleID: id})
	if err != nil || result.Legality.Formats["commander"] != card.LegalityLegal {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestCardLegalityRejectsUnknownUpstreamStatusWithoutLeakingIt(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"oracle_id":"11111111-1111-4111-8111-111111111111","name":"Example","legalities":{"standard":"sometimes"}}`), nil
	})}
	result, err := scryfall.NewAdapter("https://api.scryfall.test", client).GetCardLegality(context.Background(), card.OracleCardSelector{Name: "Example"})
	if err == nil || strings.Contains(err.Error(), "sometimes") || len(result.Legality.Formats) != 0 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestSearchCardsTranslatesPaginationAndModes(t *testing.T) {
	for _, tt := range []struct {
		mode   card.SearchResultMode
		unique string
	}{{card.SearchOracleCards, "cards"}, {card.SearchPrintings, "prints"}} {
		t.Run(string(tt.mode), func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Path != "/cards/search" || r.URL.Query().Get("q") != "t:goblin" || r.URL.Query().Get("unique") != tt.unique || r.URL.Query().Has("order") || r.URL.Query().Has("dir") || r.URL.Query().Get("page") != fmt.Sprint(calls) {
					t.Errorf("request URL = %s", r.URL)
				}
				if calls == 1 {
					return response(http.StatusOK, `{"has_more":true,"next_page":"https://search.scryfall.test/cards/search?q=t%3Agoblin&unique=`+tt.unique+`&page=2","data":[{"id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","oracle_id":"11111111-1111-4111-8111-111111111111","name":"Goblin Guide","mana_cost":"{R}","cmc":1,"type_line":"Creature — Goblin Scout","set":"zen","collector_number":"126","lang":"en"}]}`), nil
				}
				return response(http.StatusOK, `{"has_more":false,"data":[]}`), nil
			})}
			a := scryfall.NewAdapterWithOptions("https://search.scryfall.test", client, scryfall.Options{})
			first, err := a.SearchCards(context.Background(), card.SearchRequest{Query: "t:goblin", ResultMode: tt.mode, PageSize: 1})
			if err != nil || len(first.Results) != 1 || first.Next == nil || first.Results[0].Name != "Goblin Guide" {
				t.Fatalf("first=%#v err=%v", first, err)
			}
			second, err := a.SearchCards(context.Background(), card.SearchRequest{Query: "t:goblin", ResultMode: tt.mode, PageSize: 1, Continuation: first.Next})
			if err != nil || len(second.Results) != 0 || second.Next != nil {
				t.Fatalf("second=%#v err=%v", second, err)
			}
		})
	}
}

func TestSearchCardsRejectsInvalidQueryWithoutBody(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(http.StatusBadRequest, `private parser details`), nil
	})}
	_, err := scryfall.NewAdapterWithOptions("https://invalid-query.scryfall.test", client, scryfall.Options{}).SearchCards(context.Background(), card.SearchRequest{Query: "bad", ResultMode: card.SearchOracleCards, PageSize: 20})
	if !errors.Is(err, card.ErrInvalidQuery) || strings.Contains(fmt.Sprint(err), "private") {
		t.Fatalf("error=%v", err)
	}
}

func TestSearchCardsTreatsNotFoundAsSuccessfulEmptyPage(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(http.StatusNotFound, `{"details":"Your query didn't match any cards"}`), nil
	})}
	result, err := scryfall.NewAdapterWithOptions("https://empty.scryfall.test", client, scryfall.Options{}).SearchCards(context.Background(), card.SearchRequest{Query: "name:does-not-exist", ResultMode: card.SearchOracleCards, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if result.Results == nil || len(result.Results) != 0 || result.Next != nil || result.SourceURL == "" || result.FetchedAt.IsZero() || result.Stale {
		t.Fatalf("result=%#v", result)
	}
}

func TestSearchCardsBoundsPagesWithoutSkippingUpstreamResults(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"has_more":false,"data":[{"name":"One"},{"name":"Two"},{"name":"Three"}]}`), nil
	})}
	a := scryfall.NewAdapterWithOptions("https://bounded.scryfall.test", client, scryfall.Options{})
	first, err := a.SearchCards(context.Background(), card.SearchRequest{Query: "t:goblin", ResultMode: card.SearchOracleCards, PageSize: 2})
	if err != nil || len(first.Results) != 2 || first.Results[0].Name != "One" || first.Results[1].Name != "Two" || first.Next == nil {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	second, err := a.SearchCards(context.Background(), card.SearchRequest{Query: "t:goblin", ResultMode: card.SearchOracleCards, PageSize: 2, Continuation: first.Next})
	if err != nil || len(second.Results) != 1 || second.Results[0].Name != "Three" || second.Next != nil {
		t.Fatalf("second=%#v err=%v", second, err)
	}
}

func TestScryfallNotFoundIsDomainNotFound(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { return response(http.StatusNotFound, `{}`), nil })}
	_, err := scryfall.NewAdapter("https://api.scryfall.test", client).GetOracleCard(context.Background(), card.OracleCardSelector{Name: "Missing"})
	if !errors.Is(err, card.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestRulingsResolveOracleIdentityAndTranslateOfficialComments(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/cards/search":
			if r.URL.Query().Get("q") != "oracleid:"+id {
				t.Errorf("query=%s", r.URL.RawQuery)
			}
			return response(http.StatusOK, `{"data":[{"id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","oracle_id":"`+id+`"}]}`), nil
		case "/cards/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/rulings":
			return response(http.StatusOK, `{"data":[{"published_at":"2021-03-19","comment":"Official clarification."},{"published_at":"2020-01-01","comment":"Earlier ruling."}]}`), nil
		default:
			t.Fatalf("unexpected request %s", r.URL)
			return nil, nil
		}
	})}
	result, err := scryfall.NewAdapterWithOptions("https://api.scryfall.test", client, scryfall.Options{}).GetRulings(context.Background(), card.OracleCardSelector{OracleID: id})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rulings) != 2 || result.Rulings[0].PublishedAt != "2021-03-19" || result.Rulings[0].Comment != "Official clarification." {
		t.Fatalf("rulings=%#v", result.Rulings)
	}
	if !strings.HasSuffix(result.SourceURL, "/cards/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/rulings") || result.FetchedAt.IsZero() || result.Stale {
		t.Fatalf("provenance=%#v", result)
	}
}

func TestPrintingLookupBySetAndCollectorTranslatesLocalizedFacesAndOptionalFields(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.EscapedPath() != "/cards/neo/123a" {
			t.Errorf("request URL = %s", r.URL)
		}
		return response(http.StatusOK, `{
          "id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","oracle_id":"11111111-1111-4111-8111-111111111111",
          "name":"Example // Reverse","lang":"es","printed_name":"Ejemplo","printed_type_line":"Criatura","printed_text":"Texto.",
          "set":"neo","set_name":"Kamigawa: Neon Dynasty","collector_number":"123a","rarity":"rare","released_at":"2022-02-18",
          "artist":"Artist","finishes":["nonfoil","foil"],"frame":"2015","frame_effects":["showcase"],"border_color":"black","full_art":true,"textless":false,
          "card_faces":[
            {"name":"Example","printed_name":"Frente","mana_cost":"{1}","type_line":"Creature","printed_text":"Cara.","image_uris":{"normal":"https://cards.scryfall.io/normal/front.jpg"}},
            {"name":"Reverse","type_line":"Creature","image_uris":{"normal":"https://cards.scryfall.io/normal/back.jpg"}}
          ]
        }`), nil
	})}
	result, err := scryfall.NewAdapter("https://api.scryfall.test", client).GetPrinting(context.Background(), card.PrintingSelector{SetCode: "neo", CollectorNumber: "123a"})
	if err != nil {
		t.Fatal(err)
	}
	got := result.Printing
	if got.ScryfallID != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" || got.OracleID == nil || got.Language != "es" || got.CollectorNumber != "123a" || got.SetCode != "neo" {
		t.Fatalf("printing = %#v", got)
	}
	if got.PrintedName == nil || *got.PrintedName != "Ejemplo" || len(got.Faces) != 2 || *got.Faces[0].PrintedName != "Frente" || got.Faces[1].Name != "Reverse" {
		t.Fatalf("localized faces = %#v", got)
	}
	if got.Faces[0].ImageURLs["normal"] != "https://cards.scryfall.io/normal/front.jpg" || len(got.Finishes) != 2 || got.Artist == nil {
		t.Fatalf("edition facts = %#v", got)
	}
	if result.SourceURL == "" || result.FetchedAt.IsZero() || result.Stale {
		t.Fatalf("provenance = %#v", result)
	}
}

func TestPrintingLookupByScryfallIDTranslatesSingleFaceImageAndMissingOptionals(t *testing.T) {
	id := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/cards/"+id {
			t.Errorf("request URL = %s", r.URL)
		}
		return response(http.StatusOK, `{"id":"`+id+`","name":"Token","lang":"en","set":"tst","set_name":"Test","collector_number":"T-1","rarity":"common","released_at":"2026-01-01","finishes":[],"frame":"2015","border_color":"black","image_uris":{"small":"https://cards.scryfall.io/small/token.jpg"}}`), nil
	})}
	result, err := scryfall.NewAdapter("https://api.scryfall.test", client).GetPrinting(context.Background(), card.PrintingSelector{ScryfallID: id})
	if err != nil {
		t.Fatal(err)
	}
	if result.Printing.OracleID != nil || result.Printing.PrintedName != nil || result.Printing.ImageURLs["small"] == "" || result.Printing.Faces == nil {
		t.Fatalf("printing = %#v", result.Printing)
	}
}
