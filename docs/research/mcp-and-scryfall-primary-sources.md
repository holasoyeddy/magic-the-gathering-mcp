# MCP and Scryfall primary-source research

Verified on **2026-11-16**. This note uses only first-party specification,
repository, API, documentation, FAQ, and blog sources. Time-sensitive versions,
limits, and catalog contents should be rechecked before implementation.

## Requirement language

The MCP specification is normative: uppercase **MUST**, **SHOULD**, and **MAY**
have the meanings defined by BCP 14. The specification identifies its
TypeScript schema as the source of truth for protocol messages and structures.
[MCP specification][mcp-spec]

Scryfall's documentation describes service policy and operational constraints,
not an RFC-style protocol specification. This note therefore labels Scryfall
statements as **service rules** or **implementation advice**.

## Model Context Protocol

### Current published specification

The current published MCP revision is **`2026-07-28`**. MCP uses JSON-RPC 2.0;
the current base protocol is stateless and uses self-contained requests with
per-request capability declarations. [MCP specification][mcp-spec]

Normative requirements most relevant to this project include:

- Every implementation **MUST** support the base protocol, versioning, and
  message patterns, and all messages **MUST** follow JSON-RPC 2.0. Request IDs
  are strings or integers, cannot be `null`, and cannot duplicate an outstanding
  request ID. [Base protocol][mcp-base]
- A server **MUST NOT** infer protocol version, client capabilities, or client
  identity from prior requests on a connection. Each request supplies required
  `io.modelcontextprotocol/protocolVersion` and
  `io.modelcontextprotocol/clientCapabilities` values in `_meta`; state spanning
  requests **MUST** use an explicit identifier passed by the client.
  [Base protocol: statelessness and `_meta`][mcp-base]
- In the `2026-07-28` era there is no initialization handshake. Servers
  **MUST** implement `server/discover`; clients **MAY** use it before another
  request. An unsupported version produces error `-32022` with the server's
  supported versions, after which a client **SHOULD** retry a mutually supported
  version. Revisions through `2025-11-25` use the legacy handshake/session model.
  [Versioning and compatibility][mcp-versioning]
- The standard bindings are stdio (newline-delimited JSON-RPC) and Streamable
  HTTP (POST to one endpoint, with a JSON response or request-scoped SSE stream).
  Messages **MUST** be UTF-8. A custom transport **MUST** preserve the JSON-RPC
  format, message patterns, and per-request metadata model.
  [Transport specification][mcp-transports]
- A tool server **MUST** declare the `tools` capability and implement
  `tools/list`. Tool-list ordering should be deterministic and clients should
  provide human-in-the-loop confirmation; those are **SHOULD** recommendations,
  not wire-level **MUST** requirements. [Tools specification][mcp-tools]

### Official Go SDK

The official Go SDK is [`github.com/modelcontextprotocol/go-sdk`][go-sdk]. Its
primary high-level package is `mcp`; `jsonrpc` supports custom transports, while
`auth` and `oauthex` provide OAuth primitives and extensions. This package
selection is SDK implementation guidance, not a protocol requirement.

As of verification, the latest stable release is **v1.8.0**, published
**2026-09-14**. It negotiates `2026-07-28` as its newest protocol revision.
[v1.8.0 release][go-sdk-v1.8.0]

The SDK compatibility table says v1.7.0 and later support `2026-07-28`,
`2025-11-25`, `2025-06-18`, `2025-03-26`, and `2024-11-05`; client-side OAuth
support for `2025-11-25` is marked experimental. Roots, sampling, and logging
are deprecated in `2026-07-28` but retained during the compatibility window.
[Go SDK compatibility table][go-sdk-compat]

The repository's current `go.mod` declares Go **1.25.0**. That is a property of
the current SDK source, not an MCP requirement; pinning v1.8.0 should use that
tag's module metadata rather than assuming `main` remains unchanged.
[Go SDK `go.mod`][go-sdk-gomod]

## Scryfall API

### Access rules and rate limits

All calls to `api.scryfall.com` must include an accurate, application-specific
`User-Agent` and an `Accept` header. Scryfall accepts `Accept: */*` or an explicit
JSON preference. [API overview and required headers][scryfall-api]

Scryfall currently publishes these **hard service limits**:

| Endpoint | Limit | Minimum interval |
| --- | ---: | ---: |
| `/cards/search` | 2 requests/second | 500 ms |
| `/cards/named` | 2 requests/second | 500 ms |
| `/cards/collection` | 2 requests/second | 500 ms |
| `/cards/manifest` | 10 requests/minute | 6,000 ms |
| All other API methods | 10 requests/second | 100 ms |

Direct file origins under `*.scryfall.io` are not rate-limited. Excessive API
requests can return HTTP 429 and repeated overruns can result in a temporary or
permanent block. Scryfall says not to ignore 429 responses and recommends
caching downloaded data locally for at least 24 hours.
[Scryfall rate limits][scryfall-rate-limits]

The access FAQ adds operational advice: use HTTPS and TLS 1.2 or later, avoid
redundant enumeration, do not retry until requests “power through,” and do not
funnel all traffic through one VPN or relay. Its general “under 10 requests per
second” statement does not replace the stricter endpoint-specific limits above.
[Scryfall access FAQ][scryfall-access-faq]

### Bulk and incremental data options

For large datasets, large runs of simple lookups, or large image downloads,
Scryfall directs clients to bulk data instead of repeated live API calls.
[Scryfall access FAQ][scryfall-access-faq]

`GET /bulk-data` is the discovery endpoint. Download URLs contain timestamps
and change, so consumers should discover `jsonl_download_uri` instead of
hard-coding a file URL. Each current export is a gzip-compressed JSON Lines file
(`.jsonl.gz`), not a tarball, and can be streamed line-by-line. Bulk data is
collected every 12–24 hours. Prices become dangerously stale after 24 hours and
are unsuitable for storefront pricing; gameplay-only consumers will often need
updates only weekly or after set releases. These cadence statements are
Scryfall guidance, not freshness guarantees. [Bulk data docs][scryfall-bulk]

The catalog observed on 2026-11-16 exposed seven types:

| Type | Meaning |
| --- | --- |
| `oracle_cards` | One selected card object per Oracle ID |
| `unique_artwork` | Selected card objects covering unique artwork |
| `default_cards` | Every card in English, or its printed language when only one exists |
| `all_cards` | Every card in every language |
| `rulings` | All rulings, joined to cards by `oracle_id` |
| `art_tags` | Community art/illustration tags |
| `oracle_tags` | Community Oracle tags |

The authoritative live list, timestamps, sizes, and URLs are returned by the
[bulk-data API endpoint][scryfall-bulk-api].

For incremental synchronization, `/cards/manifest` returns a paginated list of
card existence, creation, and image-update metadata, 15,000 objects per page.
It is intentionally not a rich card-data feed.
[Scryfall JSONL/manifest announcement][scryfall-sync-blog]

### Card identifiers

Identifier choice encodes what “same card” means:

- `id` is Scryfall's UUID for one specific card object/printing.
- `oracle_id` is stable across reprinted editions of the same Oracle identity
  and distinguishes mechanically different cards that share a name. It is
  absent on a `reversible_card` container, where each face has its own value.
- `illustration_id` identifies artwork across reprints; newly spoiled cards may
  not have one yet.
- `arena_id`, `mtgo_id`, `mtgo_foil_id`, `multiverse_ids`, `resource_id`,
  `tcgplayer_id`, `tcgplayer_etched_id`, and `cardmarket_id` are nullable
  external-system identifiers. Many promo, token, and unusual objects lack some.
- `collector_number` is a **string**, because it can contain non-numeric
  characters.

These field semantics come from the [Card object documentation][scryfall-cards].
Single-card routes support Scryfall UUID, `(set code, collector number[, lang])`,
and Multiverse, MTGO, Arena, TCGplayer, and Cardmarket IDs.

`POST /cards/collection` resolves up to **75** submitted identifiers per request.
Accepted schemas are `id`, `mtgo_id`, `multiverse_id`, `oracle_id`,
`illustration_id`, `name`, `(name, set)`, or `(collector_number, set)`. Each input
returns at most one card; unresolved inputs appear in `not_found`. Because
missing cards shift response positions, clients should not correlate results by
array index alone. [Collection identifier schemas][scryfall-collection]

### Implementation implications (non-normative inference)

- Store `id` when exact printing identity matters; use `oracle_id` to group
  printings by Oracle identity; use `illustration_id` only for art equivalence.
- Preserve `(set, collector_number, lang)` for human/catalog imports, with the
  collector number represented as text.
- Use `/cards/collection` for bounded mixed-identifier resolution, bulk files
  for full local datasets, and `/cards/manifest` for incremental refresh checks.
- Apply a limiter per endpoint class, cache results, honor 429s with backoff,
  and do not treat the 10/s ceiling as permission to exceed stricter limits.

[mcp-spec]: https://modelcontextprotocol.io/specification/2026-07-28
[mcp-base]: https://modelcontextprotocol.io/specification/2026-07-28/basic
[mcp-versioning]: https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning
[mcp-transports]: https://modelcontextprotocol.io/specification/2026-07-28/basic/transports
[mcp-tools]: https://modelcontextprotocol.io/specification/2026-07-28/server/tools
[go-sdk]: https://github.com/modelcontextprotocol/go-sdk
[go-sdk-v1.8.0]: https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.8.0
[go-sdk-compat]: https://github.com/modelcontextprotocol/go-sdk/blob/main/README.md#version-compatibility
[go-sdk-gomod]: https://github.com/modelcontextprotocol/go-sdk/blob/main/go.mod
[scryfall-api]: https://scryfall.com/docs/api
[scryfall-rate-limits]: https://scryfall.com/docs/api/rate-limits
[scryfall-access-faq]: https://scryfall.com/docs/faqs/i-m-having-trouble-accessing-the-scryfall-api-or-i-m-blocked-17
[scryfall-bulk]: https://scryfall.com/docs/api/bulk-data
[scryfall-bulk-api]: https://api.scryfall.com/bulk-data
[scryfall-sync-blog]: https://scryfall.com/blog/two-new-ways-to-sync-scryfall-data-236
[scryfall-cards]: https://scryfall.com/docs/api/cards
[scryfall-collection]: https://scryfall.com/docs/api/cards/collection#card-identifiers
