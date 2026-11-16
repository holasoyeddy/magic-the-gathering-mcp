# 08: Package and document client setup

**What to build:** Finish the first release as a distributable single executable with protocol-safe startup, operator configuration guidance, and verified setup instructions for Codex and Claude Desktop using the same five client-neutral tools.

**Blocked by:** 02: Add exact printing lookup; 05: Search Oracle cards and printings; 06: Look up Oracle-card rulings; 07: Check Oracle-card format legality.

**Status:** ready-for-agent

- [ ] The project builds one local `stdio` executable with the official Go MCP SDK pinned to the verified release chosen by the spec.
- [ ] A subprocess smoke suite verifies protocol negotiation, deterministic discovery of all five tools, one representative tool call, cancellation or graceful EOF, and separation of protocol output from diagnostics.
- [ ] Default startup requires no configuration file and creates or uses the standard per-user cache location.
- [ ] Supported environment overrides, their defaults, and invalid-value behavior are documented.
- [ ] Installation and MCP-client configuration instructions are provided for both Codex and Claude Desktop without client-specific server behavior.
- [ ] Representative manual checks cover search, Oracle-card lookup, printing lookup, rulings, legality, empty search, exact miss, and stale fallback in both named clients.
- [ ] The default automated test suite runs without live network access; any live Scryfall conformance check is opt-in, low-volume, and rate-limit compliant.
- [ ] Contract, adapter, cache, fuzz or property, race-enabled, and subprocess tests pass as release gates.
- [ ] Runtime behavior is documented as no telemetry and no network access except the allowed Scryfall hosts.
- [ ] Documentation states the v1 exclusions, including remote transport, deck intelligence, market data, image delivery, batch lookup, and bulk-data import.
