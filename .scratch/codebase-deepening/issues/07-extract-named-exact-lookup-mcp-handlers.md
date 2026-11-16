# 07: Extract named exact-lookup MCP handlers

**What to build:** Make MCP construction readable by moving exact Oracle-card and printing behavior out of long anonymous functions and into clearly named behavior owned by the card-fact transport module.

**Blocked by:** 03: Make the card-fact adapter capability set truthful.

**Status:** ready-for-agent

- [ ] Server construction reads as a deterministic list of registered card-fact tools.
- [ ] Exact Oracle-card and printing handlers are named functions with one coherent responsibility each.
- [ ] Schema declaration, request parsing, selector validation, adapter invocation, error translation, and response shaping have clear ownership.
- [ ] Anonymous functions remain only where they are one-line, single-use, and comfortable to read inline.
- [ ] Existing tool names, schemas, ordering, results, provenance, and structured errors remain byte-for-byte compatible where contracts require stability.
- [ ] MCP contract tests remain focused on client-visible behavior rather than handler construction.
- [ ] `make check` passes.
