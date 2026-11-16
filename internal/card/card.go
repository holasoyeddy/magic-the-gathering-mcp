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

var ErrNotFound = errors.New("oracle card not found")

type Adapter interface {
	GetOracleCard(context.Context, OracleCardSelector) (OracleCardResult, error)
}
