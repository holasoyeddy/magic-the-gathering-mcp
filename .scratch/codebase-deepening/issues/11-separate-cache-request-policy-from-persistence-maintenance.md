# 11: Separate cache request policy from persistence maintenance

**What to build:** Give persistent caching a readable request path for fresh lookup, coalesced live fetch, and stale fallback while keeping record persistence, integrity, cleanup, and LRU eviction as clearly owned internal mechanics.

**Blocked by:** 10: Replace reflective cache values with typed card-fact policy.

**Status:** ready-for-agent

- [ ] The cache request happy path is visible from fresh lookup through live fetch to stale fallback.
- [ ] Single-flight ownership, waiter cancellation, and result publication have clear lifecycle ownership.
- [ ] Disk record encoding, integrity validation, atomic writes, cleanup, and eviction remain internal to the cache implementation.
- [ ] Request orchestration does not need to understand filesystem maintenance details.
- [ ] Persistence maintenance does not need to understand individual card-fact result shapes.
- [ ] Freshness, 30-day stale ceiling, diagnostic behavior, corruption recovery, restart reuse, capacity, and deterministic LRU behavior remain unchanged.
- [ ] Tests exercise request policy through the cache interface and persistence mechanics through focused internal seams.
- [ ] `make check` passes.
