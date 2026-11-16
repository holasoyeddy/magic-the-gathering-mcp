# 08: Extract the named card-search MCP handler

**What to build:** Give card search one named, readable MCP behavior so schema, defaults, validation, cursor binding, invocation, and result shaping no longer obscure server construction or the search happy path.

**Blocked by:** 07: Extract named exact-lookup MCP handlers.

**Status:** ready-for-agent

- [ ] Card search registration delegates to named behavior rather than a multi-line anonymous function.
- [ ] The happy path is visible: parse a valid search request, invoke card search, and shape the MCP result.
- [ ] Defaults, page-size bounds, result modes, opaque cursor validation, and structured error translation remain unchanged.
- [ ] Cursor policy and public error policy each have one obvious implementation home.
- [ ] Native Scryfall query syntax remains a card-search concern and does not leak into unrelated tools.
- [ ] Search contract tests retain coverage for both modes, pagination, empty results, invalid queries, cursor misuse, provenance, and bounded responses.
- [ ] `make check` passes.
