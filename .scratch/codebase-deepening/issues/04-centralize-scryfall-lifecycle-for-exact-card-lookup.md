# 04: Centralize the Scryfall lifecycle for exact card lookup

**What to build:** Establish one readable internal request lifecycle for exact Oracle-card lookup and card legality so admission, bounded attempts, cancellation, `Retry-After`, response cleanup, and shared upstream failures have one implementation home.

**Blocked by:** 03: Make the card-fact adapter capability set truthful.

**Status:** ready-for-agent

- [ ] Exact Oracle-card lookup and card legality share one internal lifecycle for limiting, request construction, attempts, waiting, cancellation, and cleanup.
- [ ] Operation code keeps a visible happy path: choose the authoritative Scryfall request, obtain a response, and translate card facts.
- [ ] Endpoint-specific meanings remain explicit and are not hidden behind generic status magic.
- [ ] Response-body and timer ownership is clear on every success, retry, failure, and cancellation path.
- [ ] Existing limiter classes, two-retry ceiling, timeout behavior, redirect protection, headers, and structured failures remain unchanged.
- [ ] Lifecycle-focused tests cover retry, `Retry-After`, cancellation, cleanup, and failure translation once.
- [ ] Exact Oracle-card and card-legality adapter tests focus on their domain translation and client-visible behavior.
- [ ] `make check` passes.
