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
