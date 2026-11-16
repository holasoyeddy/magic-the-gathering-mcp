package scryfall_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/scryfall"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
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
