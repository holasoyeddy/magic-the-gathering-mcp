# 01: Start the server with exact Oracle-card lookup

**What to build:** Deliver the first usable vertical slice: a single Go executable starts a `stdio` MCP server, advertises `get_oracle_card`, and resolves an Oracle card by exact name or Oracle ID through a replaceable card-data adapter. The client receives a curated Oracle-card result with provenance rather than raw Scryfall JSON.

**Blocked by:** None (can start immediately).

**Status:** resolved

- [x] The executable starts a valid `stdio` MCP server and advertises the tools capability without advertising out-of-scope MCP capabilities.
- [x] Tool discovery deterministically lists `get_oracle_card` with a client-neutral input schema.
- [x] The tool accepts exactly one selector: case-insensitive exact name or Oracle ID; missing, malformed, or conflicting selectors fail without calling the adapter.
- [x] A successful lookup returns the curated Oracle-card fields defined by the spec, including ordered faces for multi-face cards.
- [x] Every successful response includes `source_url`, UTC RFC 3339 `fetched_at`, and `stale` provenance fields.
- [x] Exact-name lookup never performs fuzzy matching.
- [x] The MCP handler depends on a domain-oriented card-data adapter and has no dependency on Scryfall HTTP or JSON types.
- [x] MCP contract tests drive discovery and tool invocation through the server interface with a deterministic fake adapter.
- [x] A narrow live Scryfall implementation resolves representative exact-name and Oracle-ID responses into the domain schema.
- [x] Standard output is reserved for MCP traffic and diagnostics are sent to standard error.

## Answer

Implemented the first vertical slice as a Go `stdio` MCP executable using the pinned official Go SDK v1.8.0. The server advertises only the tools capability and deterministically exposes `get_oracle_card`. Its handler validates exact-name and Oracle-ID selectors before invoking a domain adapter, returns curated Oracle-card data with ordered faces and provenance, and maps invalid selectors and exact misses to stable structured errors.

Added a narrow Scryfall adapter for exact-name and exact Oracle-ID resolution with required headers and upstream-to-domain translation. Protocol-level contract tests use the SDK's in-memory MCP transport and a deterministic fake adapter; focused adapter tests use a scripted HTTP transport and make no live network calls.

