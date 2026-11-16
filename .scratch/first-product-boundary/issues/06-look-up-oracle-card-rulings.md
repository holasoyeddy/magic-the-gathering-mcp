# 06: Look up Oracle-card rulings

**What to build:** Add `get_rulings` so an MCP client can resolve an Oracle card by exact name or Oracle ID and receive authoritative Oracle-level rulings through the same resilient adapter and cache path.

**Blocked by:** 03: Handle structured failures and Scryfall limits; 04: Cache card facts and survive Scryfall outages.

**Status:** ready-for-agent

- [ ] Tool discovery deterministically includes `get_rulings` with exact-name and Oracle-ID selectors.
- [ ] The tool accepts exactly one selector and uses the same case-insensitive exact-name and identifier-validation behavior as Oracle-card lookup.
- [ ] Successful responses contain dated Oracle-level rulings that apply across all printings.
- [ ] Rulings are ordered newest first with deterministic ordering for equal dates.
- [ ] Results include source and freshness provenance and participate in fresh-cache and stale-fallback behavior.
- [ ] Exact misses and upstream failures use the shared structured-error contract.
- [ ] The public schema contains rulings rather than printing-specific or strategic commentary.
- [ ] MCP contract tests cover name and ID lookup, multiple dates, equal-date ordering, no rulings, exact miss, invalid selector, fresh cache, and stale fallback.
- [ ] Narrow adapter tests verify Scryfall rulings translation into the domain model.

