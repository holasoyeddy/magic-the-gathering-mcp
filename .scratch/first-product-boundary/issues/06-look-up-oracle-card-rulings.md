# 06: Look up Oracle-card rulings

**What to build:** Add `get_rulings` so an MCP client can resolve an Oracle card by exact name or Oracle ID and receive authoritative Oracle-level rulings through the same resilient adapter and cache path.

**Blocked by:** 03: Handle structured failures and Scryfall limits; 04: Cache card facts and survive Scryfall outages.

**Status:** resolved

- [x] Tool discovery deterministically includes `get_rulings` with exact-name and Oracle-ID selectors.
- [x] The tool accepts exactly one selector and uses the same case-insensitive exact-name and identifier-validation behavior as Oracle-card lookup.
- [x] Successful responses contain dated Oracle-level rulings that apply across all printings.
- [x] Rulings are ordered newest first with deterministic ordering for equal dates.
- [x] Results include source and freshness provenance and participate in fresh-cache and stale-fallback behavior.
- [x] Exact misses and upstream failures use the shared structured-error contract.
- [x] The public schema contains rulings rather than printing-specific or strategic commentary.
- [x] MCP contract tests cover name and ID lookup, multiple dates, equal-date ordering, no rulings, exact miss, invalid selector, fresh cache, and stale fallback.
- [x] Narrow adapter tests verify Scryfall rulings translation into the domain model.

## Answer

Implemented `get_rulings` across the MCP, domain-adapter, Scryfall, and persistent-cache boundaries. Exact name and Oracle ID selectors resolve to Oracle-level Scryfall rulings, responses are deterministically ordered and carry provenance, and cached rulings follow the existing fresh-hit and stale-fallback policy. Contract and adapter tests cover the ticket's success, empty, ordering, selector, failure, and cache cases.

