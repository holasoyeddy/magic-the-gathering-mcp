package card

import (
	"context"
	"errors"
	"time"
)

// OracleCardSelector identifies one Oracle card by exactly one case-insensitive
// exact name or Oracle ID. Callers must leave the unused field empty.
type OracleCardSelector struct {
	Name     string
	OracleID string
}

// OracleCardFace holds the face-specific rules text of a multi-face Oracle card.
// Optional fields are nil when they do not apply to that face.
type OracleCardFace struct {
	Name       string  `json:"name"`
	ManaCost   *string `json:"mana_cost,omitempty"`
	TypeLine   *string `json:"type_line,omitempty"`
	OracleText *string `json:"oracle_text,omitempty"`
}

// OracleCard is the shared game object represented by all of its printings.
// Printing-specific details are deliberately excluded.
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

// OracleCardResult pairs an exact Oracle-card lookup with its provenance.
type OracleCardResult struct {
	Card OracleCard `json:"card"`
	Provenance
}

// Provenance describes the authoritative origin and freshness of a card fact.
// It is embedded in result values so the public JSON schema remains flat.
type Provenance struct {
	SourceURL string    `json:"source_url"`
	FetchedAt time.Time `json:"fetched_at"`
	Stale     bool      `json:"stale"`
}

// PrintingSelector identifies one printing by exactly one Scryfall ID or by a
// complete set-code and collector-number pair. Callers must not mix the forms.
type PrintingSelector struct {
	ScryfallID      string
	SetCode         string
	CollectorNumber string
}

// PrintingFace holds face-specific printed details for a multi-face printing.
// Optional localized text and image links are absent when not published.
type PrintingFace struct {
	Name        string            `json:"name"`
	PrintedName *string           `json:"printed_name,omitempty"`
	ManaCost    *string           `json:"mana_cost,omitempty"`
	TypeLine    *string           `json:"type_line,omitempty"`
	PrintedText *string           `json:"printed_text,omitempty"`
	ImageURLs   map[string]string `json:"image_urls,omitempty"`
}

// Printing is a set-specific publication of an Oracle card. Optional fields
// preserve the distinction between unavailable printing data and empty text.
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

// PrintingResult pairs an exact printing lookup with its provenance.
type PrintingResult struct {
	Printing Printing `json:"printing"`
	Provenance
}

// LegalityStatus is an authoritative format-legality classification.
type LegalityStatus string

const (
	// LegalityLegal means the Oracle card has the format's ordinary legal status.
	LegalityLegal LegalityStatus = "legal"
	// LegalityNotLegal means the Oracle card is not part of the format.
	LegalityNotLegal LegalityStatus = "not_legal"
	// LegalityRestricted means the format limits the number of copies.
	LegalityRestricted LegalityStatus = "restricted"
	// LegalityBanned means the Oracle card is explicitly prohibited.
	LegalityBanned LegalityStatus = "banned"
)

// CardLegality reports every owned format status for one Oracle card.
type CardLegality struct {
	OracleID string                    `json:"oracle_id"`
	Name     string                    `json:"name"`
	Formats  map[string]LegalityStatus `json:"formats"`
}

// LegalityResult pairs card legality with its provenance.
type LegalityResult struct {
	Legality CardLegality `json:"legality"`
	Provenance
}

// SearchResultMode selects whether card search returns one summary per Oracle
// card or one summary per printing.
type SearchResultMode string

const (
	// SearchOracleCards returns Oracle-card summaries.
	SearchOracleCards SearchResultMode = "oracle_cards"
	// SearchPrintings returns printing summaries.
	SearchPrintings SearchResultMode = "printings"
)

// SearchContinuation is an adapter-owned position in a card-search result set.
// Callers must treat Position as opaque and bind it to the original request.
type SearchContinuation struct {
	Position string `json:"position"`
}

// SearchRequest describes one bounded page of card search. Query uses the
// authoritative source's native search language; PageSize must be positive.
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

// SearchResult is one ordered page of card-search summaries. Results retain
// authoritative order, and Next is nil when no further page is available.
type SearchResult struct {
	Results []SearchSummary     `json:"results"`
	Next    *SearchContinuation `json:"next,omitempty"`
	Provenance
}

// Searcher discovers card facts using the authoritative source's native query
// language. Implementations preserve result order and honor cancellation.
type Searcher interface {
	// SearchCards returns an ordered page for a valid SearchRequest. It honors
	// cancellation and reports invalid queries and infrastructure failures with
	// the package sentinel errors.
	SearchCards(context.Context, SearchRequest) (SearchResult, error)
}

// Ruling is an official dated clarification shared by every printing of an
// Oracle card. PublishedAt is an ISO calendar date.
type Ruling struct {
	PublishedAt string `json:"published_at"`
	Comment     string `json:"comment"`
}

// RulingsResult contains rulings for one Oracle card with their provenance.
// Rulings may be empty; presentation layers own any required display ordering.
type RulingsResult struct {
	Rulings []Ruling `json:"rulings"`
	Provenance
}

// ErrNotFound means an exact card-fact lookup found no matching card.
var ErrNotFound = errors.New("oracle card not found")

var (
	// ErrInvalidQuery means a card-search query is not valid.
	ErrInvalidQuery = errors.New("invalid query")
	// ErrInvalidIdentifier means a selector or continuation is malformed.
	ErrInvalidIdentifier = errors.New("invalid identifier")
	// ErrRateLimited means the operation could not proceed within its rate limit.
	ErrRateLimited = errors.New("rate limited")
	// ErrUpstreamUnavailable means the authoritative source could not serve the operation.
	ErrUpstreamUnavailable = errors.New("upstream unavailable")
)

// Adapter provides exact card-fact lookup. Implementations must be safe for
// concurrent use, honor context cancellation, and return the sentinel failures
// above so the transport can preserve its public error contract.
type Adapter interface {
	// GetOracleCard resolves exactly one OracleCardSelector with provenance.
	GetOracleCard(context.Context, OracleCardSelector) (OracleCardResult, error)
	// GetPrinting resolves exactly one PrintingSelector with provenance.
	GetPrinting(context.Context, PrintingSelector) (PrintingResult, error)
	// GetRulings returns the possibly empty rulings for one Oracle card.
	GetRulings(context.Context, OracleCardSelector) (RulingsResult, error)
	// GetCardLegality returns the owned format statuses for one Oracle card.
	GetCardLegality(context.Context, OracleCardSelector) (LegalityResult, error)
}
