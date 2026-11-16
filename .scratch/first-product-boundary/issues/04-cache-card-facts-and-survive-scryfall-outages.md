# 04: Cache card facts and survive Scryfall outages

**What to build:** Add a persistent cache around the card-data adapter so exact MCP lookups reuse fresh card facts and can return clearly labeled stale facts during a Scryfall outage without exposing cache controls to callers.

**Blocked by:** 03: Handle structured failures and Scryfall limits.

**Status:** ready-for-agent

- [ ] Cache storage defaults to the operating system's standard per-user cache directory and supports an environment override.
- [ ] Successful domain results and provenance are stored under operation- and selector-specific versioned keys.
- [ ] Entries no older than 24 hours are returned as fresh without a Scryfall request.
- [ ] After bounded live attempts fail or are rate-limited, entries older than 24 hours but no older than 30 days are returned with `stale: true` and their original fetch timestamp.
- [ ] When no eligible entry exists, the MCP client receives the appropriate structured failure.
- [ ] Concurrent identical misses are coalesced into one upstream attempt while distinct requests remain independent.
- [ ] Cache use stays at or below 256 MiB through least-recently-used eviction.
- [ ] Expired, corrupt, and schema-incompatible entries fail safely and may be removed opportunistically.
- [ ] Cache cleanup failures are diagnostic and do not turn an otherwise successful live response into failure.
- [ ] No MCP tool exposes force refresh or other caller-controlled cache policy.
- [ ] MCP-level tests with a controllable clock cover fresh hits, stale fallback, stale rejection, original timestamps, outage behavior, and concurrent misses.
- [ ] Focused persistence tests cover restart reuse, corruption recovery, schema-version rejection, and deterministic eviction.

