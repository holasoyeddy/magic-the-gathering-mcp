package card

import (
	"context"
	"errors"
	"time"
)

type OracleCardSelector struct {
	Name     string
	OracleID string
}

type OracleCardFace struct {
	Name       string  `json:"name"`
	ManaCost   *string `json:"mana_cost,omitempty"`
	TypeLine   *string `json:"type_line,omitempty"`
	OracleText *string `json:"oracle_text,omitempty"`
}

type OracleCard struct {
	OracleID      string           `json:"oracle_id"`
	Name          string           `json:"name"`
	Layout        string           `json:"layout"`
	Faces         []OracleCardFace `json:"faces"`
	ManaCost      *string          `json:"mana_cost,omitempty"`
	ManaValue     float64          `json:"mana_value"`
	TypeLine      *string          `json:"type_line,omitempty"`
	OracleText    *string          `json:"oracle_text,omitempty"`
	Colors        []string         `json:"colors"`
	ColorIdentity []string         `json:"color_identity"`
	Keywords      []string         `json:"keywords"`
	ProducedMana  []string         `json:"produced_mana,omitempty"`
}

type OracleCardResult struct {
	Card      OracleCard `json:"card"`
	SourceURL string     `json:"source_url"`
	FetchedAt time.Time  `json:"fetched_at"`
	Stale     bool       `json:"stale"`
}

type PrintingSelector struct {
	ScryfallID      string
	SetCode         string
	CollectorNumber string
}

type PrintingFace struct {
	Name        string            `json:"name"`
	PrintedName *string           `json:"printed_name,omitempty"`
	ManaCost    *string           `json:"mana_cost,omitempty"`
	TypeLine    *string           `json:"type_line,omitempty"`
	PrintedText *string           `json:"printed_text,omitempty"`
	ImageURLs   map[string]string `json:"image_urls,omitempty"`
}

type Printing struct {
	ScryfallID      string            `json:"scryfall_id"`
	OracleID        *string           `json:"oracle_id,omitempty"`
	Name            string            `json:"name"`
	Language        string            `json:"language"`
	PrintedName     *string           `json:"printed_name,omitempty"`
	PrintedTypeLine *string           `json:"printed_type_line,omitempty"`
	PrintedText     *string           `json:"printed_text,omitempty"`
	SetCode         string            `json:"set_code"`
	SetName         string            `json:"set_name"`
	CollectorNumber string            `json:"collector_number"`
	Rarity          string            `json:"rarity"`
	ReleasedAt      string            `json:"released_at"`
	Artist          *string           `json:"artist,omitempty"`
	Finishes        []string          `json:"finishes"`
	Frame           string            `json:"frame"`
	FrameEffects    []string          `json:"frame_effects,omitempty"`
	BorderColor     string            `json:"border_color"`
	FullArt         bool              `json:"full_art"`
	Textless        bool              `json:"textless"`
	ImageURLs       map[string]string `json:"image_urls,omitempty"`
	Faces           []PrintingFace    `json:"faces"`
}

type PrintingResult struct {
	Printing  Printing  `json:"printing"`
	SourceURL string    `json:"source_url"`
	FetchedAt time.Time `json:"fetched_at"`
	Stale     bool      `json:"stale"`
}

type LegalityStatus string

const (
	LegalityLegal      LegalityStatus = "legal"
	LegalityNotLegal   LegalityStatus = "not_legal"
	LegalityRestricted LegalityStatus = "restricted"
	LegalityBanned     LegalityStatus = "banned"
)

type CardLegality struct {
	OracleID string                    `json:"oracle_id"`
	Name     string                    `json:"name"`
	Formats  map[string]LegalityStatus `json:"formats"`
}

type LegalityResult struct {
	Legality  CardLegality `json:"legality"`
	SourceURL string       `json:"source_url"`
	FetchedAt time.Time    `json:"fetched_at"`
	Stale     bool         `json:"stale"`
}

type SearchResultMode string

const (
	SearchOracleCards SearchResultMode = "oracle_cards"
	SearchPrintings   SearchResultMode = "printings"
)

type SearchContinuation struct {
	Position string `json:"position"`
}

type SearchRequest struct {
	Query        string
	ResultMode   SearchResultMode
	PageSize     int
	Continuation *SearchContinuation
}

// SearchSummary is deliberately compact: discovery identifies a card without
// duplicating the full exact-lookup representations.
type SearchSummary struct {
	OracleID        string  `json:"oracle_id,omitempty"`
	ScryfallID      string  `json:"scryfall_id,omitempty"`
	Name            string  `json:"name"`
	ManaCost        string  `json:"mana_cost,omitempty"`
	ManaValue       float64 `json:"mana_value,omitempty"`
	TypeLine        string  `json:"type_line,omitempty"`
	SetCode         string  `json:"set_code,omitempty"`
	CollectorNumber string  `json:"collector_number,omitempty"`
	Language        string  `json:"language,omitempty"`
}

type SearchResult struct {
	Results   []SearchSummary     `json:"results"`
	Next      *SearchContinuation `json:"next,omitempty"`
	SourceURL string              `json:"source_url"`
	FetchedAt time.Time           `json:"fetched_at"`
	Stale     bool                `json:"stale"`
}

type Searcher interface {
	SearchCards(context.Context, SearchRequest) (SearchResult, error)
}

type Ruling struct {
	PublishedAt string `json:"published_at"`
	Comment     string `json:"comment"`
}

type RulingsResult struct {
	Rulings   []Ruling  `json:"rulings"`
	SourceURL string    `json:"source_url"`
	FetchedAt time.Time `json:"fetched_at"`
	Stale     bool      `json:"stale"`
}

var ErrNotFound = errors.New("oracle card not found")

var (
	ErrInvalidQuery        = errors.New("invalid query")
	ErrInvalidIdentifier   = errors.New("invalid identifier")
	ErrRateLimited         = errors.New("rate limited")
	ErrUpstreamUnavailable = errors.New("upstream unavailable")
)

type Adapter interface {
	GetOracleCard(context.Context, OracleCardSelector) (OracleCardResult, error)
	GetPrinting(context.Context, PrintingSelector) (PrintingResult, error)
	GetRulings(context.Context, OracleCardSelector) (RulingsResult, error)
	GetCardLegality(context.Context, OracleCardSelector) (LegalityResult, error)
}
