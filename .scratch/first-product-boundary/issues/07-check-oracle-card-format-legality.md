# 07: Check Oracle-card format legality

**What to build:** Add `get_card_legality` so an MCP client can resolve one Oracle card and receive all authoritative upstream format statuses without performing deck validation.

**Blocked by:** 03: Handle structured failures and Scryfall limits; 04: Cache card facts and survive Scryfall outages.

**Status:** resolved

- [x] Tool discovery deterministically includes `get_card_legality` with exact-name and Oracle-ID selectors.
- [x] The tool accepts exactly one selector and follows the shared exact-name and identifier-validation behavior.
- [x] A successful response contains the complete upstream format-to-status mapping for the Oracle card.
- [x] The public schema preserves `legal`, `not_legal`, `restricted`, and `banned` without inventing additional meanings.
- [x] Results include source and freshness provenance and participate in fresh-cache and stale-fallback behavior.
- [x] Exact misses and upstream failures use the shared structured-error contract.
- [x] The tool does not accept a deck list, count copies, apply deck-construction rules, or return strategic advice.
- [x] MCP contract tests cover all status values, name and ID lookup, exact miss, invalid selector, fresh cache, and stale fallback.
- [x] Narrow adapter tests verify Scryfall legality translation without leaking the full upstream card payload.

## Answer

Implemented `get_card_legality` at the MCP boundary with exact-name and Oracle-ID selectors, curated Oracle-card legality output, shared structured errors, UTC provenance, and persistent fresh/stale cache behavior. The Scryfall adapter preserves the complete upstream format mapping and the four defined status values without exposing unrelated card fields. Contract and adapter tests cover discovery, selectors, all statuses, failures, fresh cache, and stale fallback.

