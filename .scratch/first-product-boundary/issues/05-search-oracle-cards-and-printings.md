# 05: Search Oracle cards and printings

**What to build:** Deliver flexible `search_cards` discovery through the MCP server using native Scryfall query syntax, compact bounded results, explicit Oracle-card or printing identity, and opaque continuation cursors.

**Blocked by:** 03: Handle structured failures and Scryfall limits; 04: Cache card facts and survive Scryfall outages.

**Status:** resolved

- [x] Tool discovery deterministically includes `search_cards` with `query`, optional `result_mode`, optional `page_size`, and optional `cursor` inputs.
- [x] Search accepts Scryfall query syntax without adding natural-language interpretation, fuzzy matching, similarity inference, or recommendations.
- [x] Oracle-card mode is the default and returns one compact summary per Oracle identity; printing mode preserves printing identity.
- [x] Page size defaults to 20 and accepts only values from 1 through 100.
- [x] Results preserve upstream ordering and include an opaque `next_cursor` only when another page exists.
- [x] Cursors are versioned and validated against the query, result mode, and page size; malformed, obsolete, or conflicting cursors return `invalid_identifier`.
- [x] A syntactically valid query with no matches returns a successful empty page.
- [x] A query rejected by Scryfall returns `invalid_query` without leaking the upstream error body.
- [x] Search results carry the same provenance and cache semantics as exact lookup.
- [x] Search-specific cache keys distinguish normalized query, result mode, page size, and continuation position.
- [x] MCP contract tests cover both result modes, defaults, bounds, empty results, invalid queries, cursor continuation, cursor misuse, stale fallback, and bounded response size.
- [x] Narrow adapter tests cover representative Scryfall search pagination and error translation.

## Answer

Implemented `search_cards` as a deterministic MCP tool backed by the card-data adapter. It exposes compact Oracle-card and printing summaries, validates bounded pages and versioned opaque cursors, preserves Scryfall order and query syntax, maps invalid queries to sanitized structured errors, and carries provenance through fresh and stale cache responses. The Scryfall adapter validates continuation positions and slices fixed-size upstream pages without skipping results; search cache keys bind query, result mode, page size, and continuation position.

Contract tests cover discovery, defaults, both modes, page bounds, empty pages, invalid queries, continuation and cursor conflicts, compact output, and stale fallback. Narrow adapter tests cover mode-specific upstream requests, pagination, bounded continuation, and invalid-query translation.

Scryfall's HTTP 404 no-match response is translated into the same successful empty page, including source and fetch-time provenance; adapter and MCP contract tests exercise that concrete upstream path.

