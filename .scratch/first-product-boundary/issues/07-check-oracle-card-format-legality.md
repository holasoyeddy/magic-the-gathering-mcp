# 07: Check Oracle-card format legality

**What to build:** Add `get_card_legality` so an MCP client can resolve one Oracle card and receive all authoritative upstream format statuses without performing deck validation.

**Blocked by:** 03: Handle structured failures and Scryfall limits; 04: Cache card facts and survive Scryfall outages.

**Status:** ready-for-agent

- [ ] Tool discovery deterministically includes `get_card_legality` with exact-name and Oracle-ID selectors.
- [ ] The tool accepts exactly one selector and follows the shared exact-name and identifier-validation behavior.
- [ ] A successful response contains the complete upstream format-to-status mapping for the Oracle card.
- [ ] The public schema preserves `legal`, `not_legal`, `restricted`, and `banned` without inventing additional meanings.
- [ ] Results include source and freshness provenance and participate in fresh-cache and stale-fallback behavior.
- [ ] Exact misses and upstream failures use the shared structured-error contract.
- [ ] The tool does not accept a deck list, count copies, apply deck-construction rules, or return strategic advice.
- [ ] MCP contract tests cover all status values, name and ID lookup, exact miss, invalid selector, fresh cache, and stale fallback.
- [ ] Narrow adapter tests verify Scryfall legality translation without leaking the full upstream card payload.

