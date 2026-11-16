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

func (a *Adapter) GetPrinting(ctx context.Context, selector card.PrintingSelector) (card.PrintingResult, error) {
	endpoint := a.baseURL + "/cards/" + url.PathEscape(selector.ScryfallID)
	if selector.ScryfallID == "" {
		endpoint = a.baseURL + "/cards/" + url.PathEscape(strings.ToLower(selector.SetCode)) + "/" + url.PathEscape(selector.CollectorNumber)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return card.PrintingResult{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "magic-the-gathering-mcp/0.1 (+https://github.com/holasoyeddy/magic-the-gathering-mcp)")
	response, err := a.client.Do(req)
	if err != nil {
		return card.PrintingResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return card.PrintingResult{}, card.ErrNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return card.PrintingResult{}, fmt.Errorf("scryfall returned HTTP %d", response.StatusCode)
	}
	var raw upstreamPrinting
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return card.PrintingResult{}, err
	}
	return card.PrintingResult{Printing: translatePrinting(raw), SourceURL: endpoint, FetchedAt: a.now().UTC(), Stale: false}, nil
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
