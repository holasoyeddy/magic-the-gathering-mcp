# 01: Start the server with exact Oracle-card lookup

**What to build:** Deliver the first usable vertical slice: a single Go executable starts a `stdio` MCP server, advertises `get_oracle_card`, and resolves an Oracle card by exact name or Oracle ID through a replaceable card-data adapter. The client receives a curated Oracle-card result with provenance rather than raw Scryfall JSON.

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

- [ ] The executable starts a valid `stdio` MCP server and advertises the tools capability without advertising out-of-scope MCP capabilities.
- [ ] Tool discovery deterministically lists `get_oracle_card` with a client-neutral input schema.
- [ ] The tool accepts exactly one selector: case-insensitive exact name or Oracle ID; missing, malformed, or conflicting selectors fail without calling the adapter.
- [ ] A successful lookup returns the curated Oracle-card fields defined by the spec, including ordered faces for multi-face cards.
- [ ] Every successful response includes `source_url`, UTC RFC 3339 `fetched_at`, and `stale` provenance fields.
- [ ] Exact-name lookup never performs fuzzy matching.
- [ ] The MCP handler depends on a domain-oriented card-data adapter and has no dependency on Scryfall HTTP or JSON types.
- [ ] MCP contract tests drive discovery and tool invocation through the server interface with a deterministic fake adapter.
- [ ] A narrow live Scryfall implementation resolves representative exact-name and Oracle-ID responses into the domain schema.
- [ ] Standard output is reserved for MCP traffic and diagnostics are sent to standard error.

