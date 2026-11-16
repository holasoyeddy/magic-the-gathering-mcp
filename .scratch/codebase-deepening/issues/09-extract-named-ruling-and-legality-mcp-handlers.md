# 09: Extract named ruling and card-legality MCP handlers

**What to build:** Finish the MCP transport deepening by moving ruling and card-legality behavior behind named functions, leaving server construction as a short deterministic list of five card-fact capabilities.

**Blocked by:** 08: Extract the named card-search MCP handler.

**Status:** ready-for-agent

- [ ] Ruling and card-legality registration delegates to named behavior rather than multi-line anonymous functions.
- [ ] Ruling ordering lives with ruling behavior and remains newest-first with deterministic equal-date ordering.
- [ ] Card legality continues to preserve only the four owned legality statuses.
- [ ] Shared exact-selector and public-error policies are reused without obscuring either handler's happy path.
- [ ] Server construction contains no long anonymous tool handlers and exposes the five tools in the specified order.
- [ ] MCP contract tests remain focused on client-visible ruling and card-legality behavior.
- [ ] `make check` passes.
