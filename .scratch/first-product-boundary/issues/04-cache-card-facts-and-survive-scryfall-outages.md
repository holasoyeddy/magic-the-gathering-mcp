# 04: Cache card facts and survive Scryfall outages

**What to build:** Add a persistent cache around the card-data adapter so exact MCP lookups reuse fresh card facts and can return clearly labeled stale facts during a Scryfall outage without exposing cache controls to callers.

**Blocked by:** 03: Handle structured failures and Scryfall limits.

**Status:** resolved

- [x] Cache storage defaults to the operating system's standard per-user cache directory and supports an environment override.
- [x] Successful domain results and provenance are stored under operation- and selector-specific versioned keys.
- [x] Entries no older than 24 hours are returned as fresh without a Scryfall request.
- [x] After bounded live attempts fail or are rate-limited, entries older than 24 hours but no older than 30 days are returned with `stale: true` and their original fetch timestamp.
- [x] When no eligible entry exists, the MCP client receives the appropriate structured failure.
- [x] Concurrent identical misses are coalesced into one upstream attempt while distinct requests remain independent.
- [x] Cache use stays at or below 256 MiB through least-recently-used eviction.
- [x] Expired, corrupt, and schema-incompatible entries fail safely and may be removed opportunistically.
- [x] Cache cleanup failures are diagnostic and do not turn an otherwise successful live response into failure.
- [x] No MCP tool exposes force refresh or other caller-controlled cache policy.
- [x] MCP-level tests with a controllable clock cover fresh hits, stale fallback, stale rejection, original timestamps, outage behavior, and concurrent misses.
- [x] Focused persistence tests cover restart reuse, corruption recovery, schema-version rejection, and deterministic eviction.

## Answer

Implemented a persistent card-data adapter decorator for Oracle-card and printing facts. It uses versioned normalized selector keys, integrity-checked JSON records, the OS user cache directory (overridable with `MTG_MCP_CACHE_DIR`), 24-hour fresh reuse, 30-day explicitly stale outage fallback, per-key request coalescing, and a 256 MiB deterministic least-recently-used limit. The executable now composes this cache around the live Scryfall adapter. MCP and persistence tests cover freshness, fallback, expiry, original provenance timestamps, concurrent misses, restart reuse, corrupt/incompatible records, and eviction.

