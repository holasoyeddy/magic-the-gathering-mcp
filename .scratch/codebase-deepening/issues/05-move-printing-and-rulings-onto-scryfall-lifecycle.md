# 05: Move printing and rulings onto the Scryfall lifecycle

**What to build:** Move printing lookup and Oracle-card rulings onto the reviewed Scryfall request lifecycle while preserving their identifiers, translation, ordering, provenance, and structured failures.

**Blocked by:** 04: Centralize the Scryfall lifecycle for exact card lookup.

**Status:** ready-for-agent

- [ ] Printing lookup uses the shared lifecycle for limiting, bounded attempts, cancellation, waiting, and response cleanup.
- [ ] Rulings lookup uses the same lifecycle without duplicating request policy.
- [ ] Printing retains exact Scryfall-ID and set-code/collector-number behavior, localized fields, faces, images, and provenance.
- [ ] Rulings retain exact Oracle-card selection, newest-first deterministic ordering, empty results, and provenance.
- [ ] Endpoint-specific not-found and invalid-identifier meanings remain explicit.
- [ ] Existing MCP contract behavior and cache behavior remain unchanged.
- [ ] Tests demonstrate lifecycle reuse and preserve printing and ruling translation contracts.
- [ ] `make check` passes.
