# 03: Make the card-fact adapter capability set truthful

**What to build:** Represent the complete first-release card-fact capability set at one documented domain seam so MCP transport, cache, production Scryfall access, and test adapters agree at compile time about search, exact lookup, rulings, and card legality.

**Blocked by:** 02: Make exact selectors valid domain concepts.

**Status:** ready-for-agent

- [ ] The domain seam exposes all five first-release card-fact capabilities as one coherent contract.
- [ ] Card search no longer depends on optional runtime capability checks or type assertions.
- [ ] MCP transport and cache construction cannot accept an adapter that advertises a tool it cannot execute.
- [ ] Production, cached, and deterministic test adapters satisfy the same documented capability contract.
- [ ] Test adapters contain meaningful behavior for the tests that use them rather than unrelated failure boilerplate where practical.
- [ ] All five tool schemas, ordering, responses, structured errors, and provenance remain unchanged.
- [ ] MCP contract and adapter tests pass through the consolidated seam.
- [ ] `make check` passes.
