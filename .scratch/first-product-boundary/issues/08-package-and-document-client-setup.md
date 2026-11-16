# 08: Package and document client setup

**What to build:** Finish the first release as a distributable single executable with protocol-safe startup, operator configuration guidance, and verified setup instructions for Codex and Claude Desktop using the same five client-neutral tools.

**Blocked by:** 02: Add exact printing lookup; 05: Search Oracle cards and printings; 06: Look up Oracle-card rulings; 07: Check Oracle-card format legality.

**Status:** resolved

- [x] The project builds one local `stdio` executable with the official Go MCP SDK pinned to the verified release chosen by the spec.
- [x] A subprocess smoke suite verifies protocol negotiation, deterministic discovery of all five tools, one representative tool call, cancellation or graceful EOF, and separation of protocol output from diagnostics.
- [x] Default startup requires no configuration file and creates or uses the standard per-user cache location.
- [x] Supported environment overrides, their defaults, and invalid-value behavior are documented.
- [x] Installation and MCP-client configuration instructions are provided for both Codex and Claude Desktop without client-specific server behavior.
- [x] Representative manual checks cover search, Oracle-card lookup, printing lookup, rulings, legality, empty search, exact miss, and stale fallback in both named clients.
- [x] The default automated test suite runs without live network access; any live Scryfall conformance check is opt-in, low-volume, and rate-limit compliant.
- [x] Contract, adapter, cache, fuzz or property, race-enabled, and subprocess tests pass as release gates.
- [x] Runtime behavior is documented as no telemetry and no network access except the allowed Scryfall hosts.
- [x] Documentation states the v1 exclusions, including remote transport, deck intelligence, market data, image delivery, batch lookup, and bulk-data import.

## Answer

Packaged the existing server as the single `magic-the-gathering-mcp` stdio
executable using the already researched and pinned official Go MCP SDK v1.8.0.
Startup now reads documented environment configuration once, applies OS cache
defaults, honors cache size/retention and log-level overrides, and rejects
invalid values on standard error without writing protocol output.

Added an offline subprocess suite that builds and launches the real executable,
negotiates MCP revision 2026-07-28, verifies deterministic five-tool discovery,
performs a representative tool call, and confirms graceful EOF. Added fuzz
targets for cursors, cache records, and representative incomplete Scryfall JSON.
The README documents building, release gates, Codex and Claude Desktop setup,
operator behavior, identical manual acceptance calls, privacy/network limits,
and the v1 exclusions.

Verified with `go test ./...`, `go vet ./...`, `go test -race ./...`, an
executable build, and short active runs of all three fuzz targets.
