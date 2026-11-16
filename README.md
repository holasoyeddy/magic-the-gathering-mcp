# Magic: The Gathering MCP

A local, client-neutral MCP server for authoritative Magic: The Gathering card
facts from Scryfall. It is one Go executable, communicates only over `stdio`,
and exposes these tools in a stable order:

1. `search_cards`
2. `get_oracle_card`
3. `get_printing`
4. `get_rulings`
5. `get_card_legality`

The project uses the official Go MCP SDK pinned to `v1.8.0`. The executable
supports the protocol revisions negotiated by that SDK; the release smoke test
currently verifies revision `2026-07-28`.

## Build and test

Go 1.27.2 or later is required by this module.

```sh
go test ./...
go vet ./...
go test -race ./...
mkdir -p ./bin
go build -o ./bin/magic-the-gathering-mcp ./cmd/magic-the-gathering-mcp
```

The ordinary test suite is fully offline. It uses local fixtures, fake HTTP
transports, and an executable subprocess; it never calls live Scryfall.

## Configuration

No configuration file or environment variable is required. Configuration is
read once when the process starts. Invalid values stop startup, write the error
to standard error, and produce no protocol output.

| Environment variable | Default | Behavior |
| --- | --- | --- |
| `MTG_MCP_CACHE_DIR` | OS per-user cache directory plus `magic-the-gathering-mcp` | Persistent cache directory; created with user-only permissions when absent. |
| `MTG_MCP_CACHE_MAX_BYTES` | `268435456` (256 MiB) | Positive integer byte limit; least-recently-used records are evicted. |
| `MTG_MCP_CACHE_RETENTION` | `720h` (30 days) | Go duration from `24h` through `720h`; controls stale-record eligibility. The hard stale ceiling is always 30 days. |
| `MTG_MCP_LOG_LEVEL` | `info` | One of `debug`, `info`, or `error`. `error` suppresses nonfatal cache diagnostics. |

Fresh cache records are reused for 24 hours. Records older than 24 hours but
within the configured retention can be returned only after Scryfall is
unavailable or rate-limited, and are marked `stale: true` with their original
`fetched_at` value.

Standard output is exclusively MCP's newline-delimited JSON protocol. Logs and
startup errors go to standard error. The server emits no telemetry. Its only
runtime network destination is the allowed Scryfall API host; image URLs are
returned as card facts, but image bytes are never fetched or delivered.

## Codex setup

Build the executable and register its absolute path:

```sh
go build -o "$HOME/.local/bin/magic-the-gathering-mcp" ./cmd/magic-the-gathering-mcp
codex mcp add magic-the-gathering -- "$HOME/.local/bin/magic-the-gathering-mcp"
codex mcp list
```

Codex accepts environment overrides before the command, for example:

```sh
codex mcp add --env MTG_MCP_LOG_LEVEL=error magic-the-gathering -- "$HOME/.local/bin/magic-the-gathering-mcp"
```

Restart the Codex client after changing its MCP registration if it does not
reload the server list automatically.

## Claude Desktop setup

Build the executable to a permanent location. In Claude Desktop, open
**Settings → Developer → Edit Config** and add this entry to the existing JSON
object (preserve any other configured servers):

```json
{
  "mcpServers": {
    "magic-the-gathering": {
      "command": "/absolute/path/to/magic-the-gathering-mcp",
      "args": [],
      "env": {
        "MTG_MCP_LOG_LEVEL": "info"
      }
    }
  }
}
```

Use an absolute executable path, save valid JSON, and fully restart Claude
Desktop. The server contract and behavior are identical to Codex; there are no
client-specific protocol branches.

## Manual release checks

Run the following same checks in both Codex and Claude Desktop, selecting the
named MCP tool rather than asking the server to interpret natural language:

| Check | Tool and representative arguments | Expected observation |
| --- | --- | --- |
| Search | `search_cards`: `{"query":"t:goblin","page_size":2}` | At most two compact Oracle-card summaries and provenance. |
| Oracle card | `get_oracle_card`: `{"name":"Lightning Bolt"}` | Exact canonical Oracle card, `source_url`, `fetched_at`, and `stale:false`. |
| Printing | `get_printing`: `{"set_code":"lea","collector_number":"161"}` | One printing with set/collector metadata. |
| Rulings | `get_rulings`: `{"name":"Lightning Bolt"}` | Oracle-level rulings ordered newest first. |
| Legality | `get_card_legality`: `{"name":"Lightning Bolt"}` | Complete format-to-status mapping. |
| Empty search | `search_cards`: `{"query":"name:an-impossible-card-name-xyz"}` | Successful empty `results` array, not an error. |
| Exact miss | `get_oracle_card`: `{"name":"An Impossible Card Name XYZ"}` | Structured `not_found` tool error. |
| Stale fallback | Re-run a lookup whose cache record is older than 24 hours while Scryfall is unavailable. | Same cached fact with `stale:true` and the original `fetched_at`. |

For every check, confirm the five-tool list is in the order above and that no
log line appears as tool output. These manual checks are release acceptance
evidence; the subprocess and contract suites remain the automated gates.

## v1 boundary

Version 1 deliberately excludes remote/HTTP transport, daemons, authentication,
deck intelligence or deck validation, prices and other market data, image
delivery, batch lookup, and Scryfall bulk-data import. It also does not provide
MCP resources, prompts, elicitation, or client-specific extensions.
