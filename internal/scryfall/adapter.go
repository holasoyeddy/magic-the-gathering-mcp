package scryfall

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
)

type Adapter struct {
	baseURL string
	client  *http.Client
	now     func() time.Time
}

func NewAdapter(baseURL string, client *http.Client) *Adapter {
	return &Adapter{baseURL: strings.TrimRight(baseURL, "/"), client: client, now: time.Now}
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
	endpoint := a.baseURL + "/cards/named?exact=" + url.QueryEscape(selector.Name)
	isSearch := selector.OracleID != ""
	if isSearch {
		values := url.Values{"q": {"oracleid:" + selector.OracleID}, "unique": {"cards"}}
		endpoint = a.baseURL + "/cards/search?" + values.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return card.OracleCardResult{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "magic-the-gathering-mcp/0.1 (+https://github.com/holasoyeddy/magic-the-gathering-mcp)")
	response, err := a.client.Do(req)
	if err != nil {
		return card.OracleCardResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return card.OracleCardResult{}, card.ErrNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return card.OracleCardResult{}, fmt.Errorf("scryfall returned HTTP %d", response.StatusCode)
	}
	var raw upstreamCard
	if isSearch {
		var page struct {
			Data []upstreamCard `json:"data"`
		}
		if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
			return card.OracleCardResult{}, err
		}
		if len(page.Data) != 1 {
			return card.OracleCardResult{}, card.ErrNotFound
		}
		raw = page.Data[0]
	} else if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return card.OracleCardResult{}, err
	}
	return card.OracleCardResult{Card: translate(raw), SourceURL: endpoint, FetchedAt: a.now().UTC(), Stale: false}, nil
}

func translate(raw upstreamCard) card.OracleCard {
	faces := make([]card.OracleCardFace, 0, len(raw.CardFaces))
	for _, face := range raw.CardFaces {
		faces = append(faces, card.OracleCardFace{Name: face.Name, ManaCost: face.ManaCost, TypeLine: face.TypeLine, OracleText: face.OracleText})
	}
	return card.OracleCard{
		OracleID: raw.OracleID, Name: raw.Name, Layout: raw.Layout, Faces: faces,
		ManaCost: raw.ManaCost, ManaValue: raw.CMC, TypeLine: raw.TypeLine, OracleText: raw.OracleText,
		Colors: nonNil(raw.Colors), ColorIdentity: nonNil(raw.ColorIdentity), Keywords: nonNil(raw.Keywords), ProducedMana: raw.ProducedMana,
	}
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
