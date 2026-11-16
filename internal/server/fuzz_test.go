package server

import "testing"

func FuzzCursorDecodeNeverPanics(f *testing.F) {
	f.Add("")
	f.Add("eyJ2IjoxLCJxIjoidDpn b2JsaW4ifQ")
	f.Add(encodeCursor(searchCursor{Version: 1, Query: "t:goblin", Mode: "oracle_cards", PageSize: 20, Position: "page-2"}))
	f.Fuzz(func(t *testing.T, value string) {
		cursor, err := decodeCursor(value)
		if err == nil && encodeCursor(cursor) == "" {
			t.Fatal("valid cursor did not re-encode")
		}
	})
}
