# 01: Document exported card-fact contracts

**What to build:** Make the public Go surface readable without implementation archaeology by documenting every exported card-fact type and function, including the invariants and operational behavior a caller must understand.

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

- [ ] Every exported function has a Go documentation comment that begins with its name.
- [ ] Every exported domain type, adapter, option, and sentinel failure has a useful documentation comment.
- [ ] Comments describe relevant selector invariants, ordering, provenance, error behavior, cancellation, concurrency, and resource ownership.
- [ ] Comments use the project vocabulary from `GLOSSARY.md` and do not expose Scryfall or MCP implementation details through the domain seam.
- [ ] Documentation explains contracts rather than restating declarations.
- [ ] Runtime behavior and public schemas remain unchanged.
- [ ] The pull request contains documentation-only changes and passes `make check`.
