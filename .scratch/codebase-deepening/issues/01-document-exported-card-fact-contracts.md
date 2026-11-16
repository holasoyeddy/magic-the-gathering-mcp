# 01: Document exported card-fact contracts

**What to build:** Make the public Go surface readable without implementation archaeology by documenting every exported card-fact type and function, including the invariants and operational behavior a caller must understand.

**Blocked by:** None (can start immediately).

**Status:** resolved

- [x] Every exported function has a Go documentation comment that begins with its name.
- [x] Every exported domain type, adapter, option, and sentinel failure has a useful documentation comment.
- [x] Comments describe relevant selector invariants, ordering, provenance, error behavior, cancellation, concurrency, and resource ownership.
- [x] Comments use the project vocabulary from `GLOSSARY.md` and do not expose Scryfall or MCP implementation details through the domain seam.
- [x] Documentation explains contracts rather than restating declarations.
- [x] Runtime behavior and public schemas remain unchanged.
- [x] The pull request contains documentation-only changes and passes `make check`.

## Answer

Documented the exported card-fact contracts across the domain, cache, live adapter, MCP construction, and process configuration surfaces in implementation commit `7c892ab`. The change passed the two-axis code review and `make check`, then merged into `main` as `67a6a55`. The complete offline test suite passed again after the merge.
