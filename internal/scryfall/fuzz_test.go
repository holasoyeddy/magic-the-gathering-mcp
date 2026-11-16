package scryfall

import (
	"encoding/json"
	"testing"

	"github.com/holasoyeddy/magic-the-gathering-mcp/internal/card"
)

func FuzzRepresentativeCardJSONMissingOptionalFieldsNeverPanics(f *testing.F) {
	f.Add([]byte(`{"id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","oracle_id":"11111111-1111-4111-8111-111111111111","name":"Example","layout":"normal","cmc":1}`))
	f.Add([]byte(`{}`))
	f.Add([]byte("not-json"))
	f.Fuzz(func(t *testing.T, data []byte) {
		var raw upstreamCard
		if json.Unmarshal(data, &raw) == nil {
			got := translate(raw)
			if got.Faces == nil || got.Colors == nil || got.ColorIdentity == nil || got.Keywords == nil {
				t.Fatalf("missing fields must produce bounded empty collections: %#v", got)
			}
			_ = searchSummary(raw, card.SearchOracleCards)
		}
	})
}
