# 05: Search Oracle cards and printings

**What to build:** Deliver flexible `search_cards` discovery through the MCP server using native Scryfall query syntax, compact bounded results, explicit Oracle-card or printing identity, and opaque continuation cursors.

**Blocked by:** 03: Handle structured failures and Scryfall limits; 04: Cache card facts and survive Scryfall outages.

**Status:** ready-for-agent

- [ ] Tool discovery deterministically includes `search_cards` with `query`, optional `result_mode`, optional `page_size`, and optional `cursor` inputs.
- [ ] Search accepts Scryfall query syntax without adding natural-language interpretation, fuzzy matching, similarity inference, or recommendations.
- [ ] Oracle-card mode is the default and returns one compact summary per Oracle identity; printing mode preserves printing identity.
- [ ] Page size defaults to 20 and accepts only values from 1 through 100.
- [ ] Results preserve upstream ordering and include an opaque `next_cursor` only when another page exists.
- [ ] Cursors are versioned and validated against the query, result mode, and page size; malformed, obsolete, or conflicting cursors return `invalid_identifier`.
- [ ] A syntactically valid query with no matches returns a successful empty page.
- [ ] A query rejected by Scryfall returns `invalid_query` without leaking the upstream error body.
- [ ] Search results carry the same provenance and cache semantics as exact lookup.
- [ ] Search-specific cache keys distinguish normalized query, result mode, page size, and continuation position.
- [ ] MCP contract tests cover both result modes, defaults, bounds, empty results, invalid queries, cursor continuation, cursor misuse, stale fallback, and bounded response size.
- [ ] Narrow adapter tests cover representative Scryfall search pagination and error translation.

