# 06: Move card search onto the Scryfall lifecycle

**What to build:** Complete the Scryfall lifecycle deepening by moving card search onto it without changing native query semantics, upstream ordering, bounded pagination, cursors, caching, or search-specific failures.

**Blocked by:** 05: Move printing and rulings onto the Scryfall lifecycle.

**Status:** ready-for-agent

- [ ] Card search uses the shared lifecycle for limiting, bounded attempts, cancellation, waiting, and response cleanup.
- [ ] Native Scryfall query syntax and upstream result order remain unchanged.
- [ ] A Scryfall no-results response remains a successful empty page with provenance.
- [ ] Invalid queries remain `invalid_query`; malformed or conflicting cursors remain `invalid_identifier`.
- [ ] Oracle-card and printing result modes, page-size bounds, continuation, and stale fallback remain unchanged.
- [ ] Superseded request and retry paths are removed after all card-fact operations use the shared lifecycle.
- [ ] Lifecycle tests own common request policy while card-search tests own search-specific translation and pagination behavior.
- [ ] `make check` passes.
