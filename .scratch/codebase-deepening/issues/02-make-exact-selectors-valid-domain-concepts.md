# 02: Make exact selectors valid domain concepts

**What to build:** Ensure exact Oracle-card and printing lookups cross the domain seam only as valid, unambiguous selectors so cache and Scryfall adapters no longer reinterpret empty, partial, malformed, or conflicting identifier combinations.

**Blocked by:** 01: Document exported card-fact contracts.

**Status:** ready-for-agent

- [ ] Oracle-card selection represents exactly one case-insensitive exact name or valid Oracle ID.
- [ ] Printing selection represents exactly one valid Scryfall ID or one complete set-code and collector-number pair.
- [ ] Invalid selector states are rejected at the MCP edge before adapter or cache access.
- [ ] Cache keys and Scryfall routes consume validated domain selectors without choosing between conflicting fields.
- [ ] Existing MCP schemas, exact-match behavior, structured errors, and client-visible results remain unchanged.
- [ ] Contract tests cover every valid selector form and representative invalid, partial, malformed, and conflicting forms.
- [ ] Adapter and cache tests prove that downstream modules receive only valid selectors.
- [ ] `make check` passes.
