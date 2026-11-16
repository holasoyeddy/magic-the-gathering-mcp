# First Product Boundary

## Purpose

Build a local, single-user MCP server that gives standards-compliant MCP clients reliable access to authoritative Magic: The Gathering card facts from Scryfall.

The first release establishes trustworthy retrieval. It does not provide deck intelligence or market data.

## Consumers

- Support any standards-compliant MCP client without client-specific behavior.
- Treat Codex and Claude Desktop as named compatibility targets.
- Require representative tool calls to work unchanged in both clients.
- Maintain protocol-level tests that do not depend on either named client.

## Runtime boundary

- Ship one Go executable.
- Use local `stdio` transport only.
- Expose MCP tools only; do not expose resources, prompts, elicitation, or client-specific extensions.
- Reserve standard output for MCP protocol messages and send diagnostics to standard error.
- Require no configuration file.
- Allow environment-variable overrides only for exceptional settings such as cache location and log level.
- Collect no telemetry.
- Make no runtime network requests except to Scryfall.
- Defer Streamable HTTP, hosted operation, authentication, tenancy, and daemon processes.

## Initial tools

### `search_cards`

Discover Oracle cards or printings using Scryfall's native query syntax.

- Accept a Scryfall query, not natural-language instructions.
- Return Oracle-card summaries by default.
- Support an explicit printing-result mode.
- Return compact summaries; callers use exact-lookup tools for full records.
- Use cursor pagination with a default page size of 20 and a maximum of 100.
- Return an opaque `next_cursor` when another page exists.
- Return an empty page for a valid query with no matches.
- Treat explicit factual predicates and upstream ordering as search.
- Do not infer similarity, fitness, or recommendations.

### `get_oracle_card`

Resolve one Oracle card by case-insensitive exact name or Oracle ID.

- Do not silently perform fuzzy matching.
- Return `not_found` when an exact name or ID cannot be resolved.
- Return canonical English rules identity.
- Include name, layout, ordered faces, mana cost, mana value, type line, Oracle text, colors, color identity, keywords, and produced mana when applicable.
- Keep only fields shared across all faces at the top level; face-specific fields belong in the ordered `faces` array.
- Do not embed rulings or legality, which have dedicated tools.

### `get_printing`

Resolve one printing by Scryfall ID or by set code and collector number.

- Include Scryfall ID, Oracle ID, language, localized name/type line/printed text when supplied, set identity, collector number, rarity, release date, artist, finishes, frame treatment, and image URLs when applicable.
- Do not duplicate the complete Oracle-card record.
- Represent multi-face layouts with an ordered `faces` array.
- Return Scryfall-hosted image URLs only; do not download, cache, proxy, or embed image bytes.

### `get_rulings`

Resolve an Oracle card by case-insensitive exact name or Oracle ID and return its Oracle-level rulings.

- Apply rulings to the Oracle card across all printings.
- Order rulings newest first.

### `get_card_legality`

Resolve an Oracle card by case-insensitive exact name or Oracle ID and return every upstream format status.

- Preserve the statuses `legal`, `not_legal`, `restricted`, and `banned`.
- Report only single-card legality.
- Do not validate a deck or apply deck-construction rules.

## Public response contract

- Return curated, server-owned schemas rather than raw Scryfall JSON.
- Preserve upstream identifiers and source links.
- Include `source_url`, `fetched_at`, and `stale` on every successful response.
- Preserve the original upstream-fetch time when serving cached data.
- Use stable structured error codes with a concise human-readable message.
- Expected codes include `not_found`, `invalid_query`, `invalid_identifier`, `rate_limited`, and `upstream_unavailable`.
- Do not expose a caller-controlled cache bypass.

## Scryfall access and cache

- Use one shared Scryfall client for all tools.
- Send the application-specific headers required by Scryfall.
- Centrally enforce Scryfall endpoint rate limits.
- Honor `Retry-After`.
- Attempt at most two bounded retries for transient upstream failures.
- Persist responses in the operating system's standard per-user cache directory.
- Permit a cache-directory override through an environment variable.
- Treat entries as fresh for 24 hours.
- When live access fails or is rate-limited, return an older cached response when one exists and mark it `stale: true`.
- Keep stale entries eligible for fallback for 30 days.
- Apply least-recently-used eviction above 256 MiB.
- After retries, return `upstream_unavailable` when no eligible cached response exists.
- Defer Scryfall bulk-data imports.

## Product boundary

The server answers card-fact questions whose results are directly attributable to Scryfall. Explicit user-supplied predicates remain card search even when they filter or order many cards.

The following are inside the first boundary:

- Oracle-card discovery and exact lookup
- Printing-specific lookup and metadata
- Canonical rules text and card characteristics
- Official rulings
- Single-card format-legality statuses
- Upstream image URLs

The following are outside the first boundary:

- Natural-language interpretation inside the server
- Fuzzy-name and autocomplete tools
- Batch, collection, or deck-list lookup
- Whole-deck legality and deck-construction validation
- Similarity inference
- Synergy or weakness analysis
- Scoring, substitutions, recommendations, and strategic advice
- Prices, vendors, and purchase links
- Image storage or delivery

## Acceptance criteria

The first product boundary is delivered when:

1. The five initial tools expose client-neutral schemas over `stdio`.
2. Representative success, empty-result, invalid-input, exact-not-found, stale-cache, and upstream-failure cases pass protocol-level tests.
3. The same representative tool calls work from Codex and Claude Desktop without client-specific branches.
4. Multi-face and non-English printing fixtures preserve their domain distinctions.
5. Search results remain within the requested page bound and continue through opaque cursors.
6. Standard output contains only valid MCP protocol traffic.
7. Runtime network tests observe calls only to Scryfall.
8. No tool performs an excluded deck-intelligence or market-data operation.

## Deferred decisions

The following require a new product decision before implementation:

- Streamable HTTP or hosted deployment
- Multi-user access, authentication, or tenancy
- Bulk-data import or an offline-first dataset
- Batch or collection resolution
- Deck intelligence or deck validation
- Market data
- Additional MCP capability types
