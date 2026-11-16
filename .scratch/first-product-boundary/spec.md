# Build the First Card-Facts MCP Server

Status: ready-for-agent

## Problem Statement

An MCP client cannot currently ask this repository for authoritative Magic: The Gathering card facts. The repository contains product decisions and domain language, but no server, tool contracts, Scryfall integration, cache, or executable. A first implementation must provide useful card discovery and exact lookup without coupling callers to raw Scryfall payloads or allowing deck intelligence, market data, or client-specific behavior to leak into the product boundary.

The build must also remain dependable when Scryfall is slow, rate-limited, or temporarily unavailable. Its behavior needs to be testable without live network access. The highest-value testing seam is the MCP server interface: tests should invoke the same tool contracts that Codex, Claude Desktop, and other MCP clients invoke, while supplying a replaceable card-data adapter instead of calling Scryfall.

## Solution

Build a single local Go executable that serves five client-neutral MCP tools over `stdio`: `search_cards`, `get_oracle_card`, `get_printing`, `get_rulings`, and `get_card_legality`.

The MCP server owns validation, stable public schemas, pagination, structured errors, and provenance metadata. Application services implement the product rules behind those tools. They depend on a narrow card-data adapter rather than a concrete Scryfall client. The production adapter uses Scryfall's live API through shared rate limiting, bounded retries, and a persistent disk cache. Tests replace that adapter with deterministic in-memory behavior and exercise the server through MCP requests.

The server returns curated Oracle-card, printing, ruling, and legality representations. It accepts native Scryfall syntax for discovery but does not expose raw Scryfall JSON. It reports cache age and stale fallback explicitly. It does not interpret natural language, perform deck analysis, expose market data, or make network requests to any service other than Scryfall.

## User Stories

1. As an MCP client user, I want to discover cards with Scryfall query syntax, so that I can use Scryfall's established search language through my client.
2. As an MCP client user, I want search to return Oracle cards by default, so that reprints do not overwhelm ordinary discovery results.
3. As an MCP client user, I want to request printing results explicitly, so that I can search edition-specific properties when needed.
4. As an MCP client user, I want compact search summaries, so that broad searches do not consume unnecessary model context.
5. As an MCP client user, I want bounded search pages, so that one call cannot produce an unmanageably large response.
6. As an MCP client user, I want an opaque continuation cursor, so that I can retrieve more results without depending on Scryfall pagination internals.
7. As an MCP client user, I want a valid search with no matches to return an empty page, so that absence is not confused with a server failure.
8. As an MCP client user, I want invalid Scryfall syntax identified as `invalid_query`, so that I can correct the query.
9. As an MCP client user, I want to retrieve an Oracle card by exact name, so that I can obtain canonical gameplay identity and rules text.
10. As an MCP client user, I want exact-name matching to be case-insensitive but not fuzzy, so that lookup remains deterministic.
11. As an MCP client user, I want to retrieve an Oracle card by Oracle ID, so that I can preserve identity across printings.
12. As an MCP client user, I want multi-face Oracle cards represented as ordered faces, so that transforming, modal, split, adventure, and other layouts retain their structure.
13. As an MCP client user, I want canonical English Oracle data, so that rules identity is consistent across localized printings.
14. As an MCP client user, I want to retrieve a printing by Scryfall ID, so that I can resolve one exact edition object.
15. As an MCP client user, I want to retrieve a printing by set code and collector number, so that human catalog identifiers work without a Scryfall UUID.
16. As an MCP client user, I want collector numbers preserved as text, so that non-numeric collector numbers remain valid.
17. As an MCP client user, I want printing language and localized text when available, so that non-English printings are represented faithfully.
18. As an MCP client user, I want printing metadata such as set, rarity, release date, artist, finishes, and frame treatment, so that edition-specific questions can be answered.
19. As an MCP client user, I want Scryfall-hosted image URLs, so that I can reference artwork without image bytes inflating MCP responses.
20. As an MCP client user, I want an exact lookup miss identified as `not_found`, so that I can distinguish it from an empty discovery result.
21. As an MCP client user, I want official rulings resolved by exact name or Oracle ID, so that the answer applies across all printings.
22. As an MCP client user, I want rulings ordered newest first, so that the most recent clarification is easiest to find.
23. As an MCP client user, I want all current format-legality statuses for one Oracle card, so that I do not need one call per format.
24. As an MCP client user, I want legality statuses preserved as `legal`, `not_legal`, `restricted`, or `banned`, so that upstream meaning is not blurred.
25. As an MCP client user, I want every successful response to identify its source and fetch time, so that I can assess provenance and freshness.
26. As an MCP client user, I want stale cached responses clearly labeled, so that degraded operation never masquerades as fresh data.
27. As an MCP client user, I want stable structured error codes, so that my client does not have to parse prose or raw Scryfall errors.
28. As an MCP client user, I want the same tool contracts in Codex and Claude Desktop, so that my workflow is portable between clients.
29. As a local operator, I want the server to work without a configuration file, so that installation has minimal setup.
30. As a local operator, I want the cache to use the standard per-user cache location, so that repository and working directories stay clean.
31. As a local operator, I want environment overrides for cache location and log level, so that tests and unusual installations remain manageable.
32. As a local operator, I want diagnostics written to standard error, so that logging cannot corrupt the MCP protocol stream.
33. As a local operator, I want no telemetry, so that the server's runtime behavior remains private and auditable.
34. As a local operator, I want cached facts reused for 24 hours, so that ordinary use respects Scryfall's service guidance.
35. As a local operator, I want eligible stale data during an upstream outage, so that temporary Scryfall failures do not make known facts unavailable.
36. As a local operator, I want cache storage bounded, so that long-running use cannot grow disk usage indefinitely.
37. As a Scryfall service operator, I want all requests centrally rate-limited and retried conservatively, so that this server behaves as a responsible API consumer.
38. As a maintainer, I want tool behavior tested at the MCP interface, so that tests protect client-observable contracts rather than internal implementation details.
39. As a maintainer, I want Scryfall behind a replaceable adapter, so that MCP behavior can be tested deterministically without network access.
40. As a maintainer, I want upstream payloads translated at one boundary, so that Scryfall schema changes do not leak throughout the codebase.
41. As a maintainer, I want the cache and live adapter composed outside the MCP server, so that transport behavior is independent of data-source mechanics.
42. As a maintainer, I want deterministic tool-list ordering and schemas, so that compatibility checks and snapshots are stable.
43. As a maintainer, I want one executable and no daemon, so that distribution and rollback remain simple.
44. As a maintainer, I want client cancellation propagated through server, cache, limiter, and network calls, so that abandoned work releases resources promptly.
45. As a maintainer, I want concurrent identical cache misses coalesced, so that simultaneous requests do not multiply upstream traffic.

## Implementation Decisions

- Build one Go executable using the official Go MCP SDK. Pin the dependency to the verified stable release recorded in project research rather than tracking a moving branch. Support the protocol revisions negotiated by that SDK; do not implement client-specific protocol branches.
- The executable runs a `stdio` server only. Standard output is owned exclusively by newline-delimited MCP protocol traffic. Logs and diagnostics go to standard error.
- Advertise the MCP tools capability and expose exactly five tools in deterministic order: `search_cards`, `get_oracle_card`, `get_printing`, `get_rulings`, and `get_card_legality`. Do not advertise resources, prompts, elicitation, or client-specific extensions.
- Separate the system into four responsibilities: MCP transport and tool handlers; application services and public domain schemas; a replaceable card-data adapter; and production infrastructure for Scryfall access, caching, limiting, and persistence.
- Treat the MCP server interface as the primary product boundary. Tool handlers validate requests, invoke application services, and translate results into stable MCP content and structured errors. They do not construct Scryfall URLs, parse upstream JSON, or access the disk cache directly.
- Define a narrow card-data adapter in domain terms. It supports native-query search, exact Oracle-card resolution, exact printing resolution, rulings retrieval, and legality-bearing Oracle data. Its methods accept cancellation context and return domain records plus source/freshness metadata or typed domain failures.
- Keep the adapter independent of MCP types. This allows another upstream or an offline implementation to replace Scryfall without rewriting tool handlers, and prevents protocol concerns from contaminating data access.
- Implement the production adapter as a composition of a live Scryfall gateway and a cache decorator. The live gateway alone owns HTTP routes, required headers, upstream JSON types, upstream error interpretation, and translation into domain records.
- Use one shared HTTP client and one shared limiter registry. Apply the stricter 500 ms minimum interval to search and named-lookup endpoint classes and the appropriate published limit to other endpoints. Honor `Retry-After` when Scryfall returns 429.
- Retry only transient failures and at most twice. Do not retry invalid requests, definitive not-found responses, or other non-transient 4xx responses. Every retry remains cancellation-aware and bounded by the request deadline.
- Coalesce concurrent identical cache misses before calling Scryfall. Cache keys include the operation, normalized identifiers or exact query parameters, result mode, and page position; semantically different requests must never collide.
- Persist cache entries in the operating system's standard per-user cache directory. Permit an environment override for the cache directory. Cache records contain a schema version, key, serialized domain result, source URL, fetch timestamp, last-access timestamp, and integrity/version metadata sufficient to reject incompatible or corrupt records.
- Treat an entry as fresh for 24 hours. A fresh entry is returned without a network call. An entry older than 24 hours but no older than 30 days is eligible only for fallback after the live request and bounded retries fail or are rate-limited.
- Mark fallback responses `stale: true` and preserve the original fetch timestamp. Fresh live and fresh-cache responses use `stale: false`.
- Keep cache usage at or below 256 MiB using least-recently-used eviction. Expired entries and corrupt or schema-incompatible entries may be removed opportunistically. Cache cleanup failure is diagnostic and must not corrupt a successful live response.
- Do not expose caller-controlled force refresh. The cache policy is owned by the server.
- Use curated public schemas rather than raw Scryfall JSON. Public schemas use project glossary terms: Oracle card, printing, card fact, ruling, and card legality.
- Every successful tool response includes a data payload plus `source_url`, `fetched_at`, and `stale`. Timestamps use UTC RFC 3339. Source URLs identify the authoritative Scryfall API request, not local cache files.
- Model multi-face objects with an ordered `faces` collection. Keep only genuinely shared fields at the top level. Preserve missing optional upstream fields as absent rather than fabricating empty values.
- Model collector number as text. Preserve Scryfall UUIDs and Oracle IDs as strings validated for UUID shape. Preserve set code in a normalized lowercase representation while accepting case-insensitive input.
- `search_cards` accepts `query`, optional `result_mode`, optional `page_size`, and optional `cursor`. `result_mode` defaults to Oracle cards and permits printings. `page_size` defaults to 20 and is limited to 1 through 100.
- A search cursor is opaque, versioned, and tamper-detectable or fully server-validated. It binds continuation to the normalized query, result mode, and page size. A cursor reused with conflicting parameters returns `invalid_identifier`. Do not expose Scryfall page URLs.
- Search returns compact summaries, an ordered result collection, and optional `next_cursor`. A syntactically valid search with no matches returns an empty result collection. Scryfall query rejection maps to `invalid_query`.
- Oracle-card search mode deduplicates by Oracle identity. Printing mode preserves printing identity. Ordering follows Scryfall's result order for the submitted query.
- `get_oracle_card` accepts exactly one selector: case-insensitive exact name or Oracle ID. Multiple or missing selectors return `invalid_identifier`. It never falls back to fuzzy matching.
- A full Oracle-card result includes Oracle ID, canonical English name, layout, ordered faces, mana cost, mana value, type line, Oracle text, colors, color identity, keywords, and produced mana when applicable. It does not embed rulings, legality, market data, or all printings.
- `get_printing` accepts exactly one selector: Scryfall ID, or the pair of set code and collector number. Multiple, partial, or missing selectors return `invalid_identifier`.
- A full printing result includes Scryfall ID, Oracle ID when present, language, localized printed fields when present, set identity, collector number, rarity, release date, artist, finishes, frame treatment, and image URLs when applicable. It references but does not duplicate the full Oracle-card result.
- Return Scryfall-hosted image URLs as data. Never fetch, proxy, embed, persist, or transform image content.
- `get_rulings` accepts exactly one Oracle-card selector: case-insensitive exact name or Oracle ID. Return dated Oracle-level rulings newest first, using a deterministic secondary ordering when dates are equal.
- `get_card_legality` accepts exactly one Oracle-card selector: case-insensitive exact name or Oracle ID. Return the complete upstream format-to-status mapping for that Oracle card, preserving `legal`, `not_legal`, `restricted`, and `banned`.
- Expected failures use stable server-owned codes: `not_found`, `invalid_query`, `invalid_identifier`, `rate_limited`, and `upstream_unavailable`. Include a concise human-readable message and no raw upstream body. Unexpected internal failures use a generic internal error and log diagnostic detail to standard error.
- `not_found` is reserved for exact resolution. Empty search is successful. `invalid_query` means Scryfall rejected search semantics. `invalid_identifier` covers malformed or conflicting selectors and unusable cursors. `rate_limited` covers a request that cannot be admitted within its bounded execution window. `upstream_unavailable` covers exhausted transient failures without eligible stale data.
- Propagate cancellation from the MCP request through application services, cache coordination, limiter waits, and HTTP requests. Cancellation stops retries and returns the protocol-appropriate cancellation outcome rather than an upstream error.
- Configuration is environment-based and read once during startup. Defaults require no setup. Reject invalid cache-size, retention, or log-level overrides with a startup error; do not silently substitute values.
- Send an accurate application-specific `User-Agent` and JSON-compatible `Accept` header on every Scryfall request. Do not send user prompts, client identity, or unrelated local data upstream.
- The only runtime network destination is Scryfall's documented API host. Redirect handling must not silently send requests or headers to unrelated hosts.
- No authentication or authorization exists in v1 because the server is a local single-user `stdio` process. Do not add secrets or API-key storage.
- Version public tool schemas with the executable release. Additive optional fields may be introduced compatibly; renaming, removal, meaning changes, or new required fields require an explicit compatibility decision and corresponding contract-test update.
- Keep the existing glossary and ADRs authoritative. Material changes to identity, source strategy, query syntax, public schema ownership, or the product boundary require documentation updates before implementation proceeds.

## Testing Decisions

- The main seam is the MCP server interface. Most tests create a server with a deterministic fake card-data adapter, send protocol-level discovery/list/call requests, and assert only client-observable tool schemas, structured content, errors, ordering, and metadata.
- A good test describes externally visible behavior and remains valid if handlers, application services, cache implementation, or adapter composition are refactored. Tests must not assert internal call graphs, private type shapes, helper invocation counts, or concrete module layout unless the behavior itself is a concurrency or rate-limit contract.
- Contract coverage includes deterministic tool listing; each request schema; successful responses; optional and missing fields; invalid selectors; invalid queries; exact misses; empty searches; pagination; cursor misuse; multi-face cards; non-English printings; rulings ordering; legality statuses; provenance; stale markers; cancellation; and stable error mapping.
- The fake adapter returns domain records, typed failures, delays, and cancellation behavior. It records only enough interaction data to assert externally meaningful facts such as “a fresh cache hit caused no upstream request” or “concurrent identical misses produced one upstream attempt.” It does not mimic Scryfall HTTP or raw JSON.
- Cache behavior is tested through the MCP seam wherever practical by using a controllable clock and temporary cache store. Cover fresh hits, stale fallback, stale rejection after 30 days, original fetch timestamps, no force refresh, corrupt-entry recovery, version incompatibility, and least-recently-used eviction at the configured limit.
- Add narrow adapter contract tests using recorded, minimal Scryfall fixtures. These verify translation from upstream JSON and HTTP status classes into domain records and typed failures. They are the only tests allowed to depend on Scryfall field names or endpoint shapes.
- Add narrow limiter/retry tests with a fake clock and scripted HTTP transport. Verify endpoint-class spacing, `Retry-After`, the two-retry ceiling, non-retryable responses, cancellation during waits, and no retry storm.
- Add one subprocess-level `stdio` smoke suite against the built executable. Verify protocol negotiation supported by the pinned SDK, valid tool discovery, one representative call, graceful EOF/cancellation, and that standard output contains only protocol messages while diagnostics remain on standard error.
- Add compatibility checks for the MCP protocol revisions supported by the pinned SDK. Codex and Claude Desktop manual acceptance use the same documented tool calls and expected shapes; no client-specific golden output is maintained.
- Do not make ordinary automated tests call live Scryfall. An opt-in live conformance check may verify headers and one low-volume representative lookup, but it must be excluded from the default test command, obey rate limits, tolerate service availability, and never be the sole verification of behavior.
- Add race-enabled tests for concurrent tool calls, identical cache misses, cache reads during eviction, cancellation, and shutdown. Passing the Go race detector is a merge gate.
- Add fuzz or property coverage for cursor decoding, selector validation, cache-record decoding, and representative upstream JSON with missing optional fields. Invalid input must never panic or expose raw internal errors.
- Add response-size tests showing that page-size limits are enforced and search summaries remain bounded. Add cache-capacity tests using deterministic record sizes rather than writing hundreds of real card payloads.
- Add a network-boundary test around the production HTTP transport to reject redirects or requests outside allowed Scryfall hosts.
- Prior art: the repository has no source or test suite yet. The MCP contract harness established here becomes the canonical testing pattern; later features should extend this seam before introducing new lower-level seams.
- Completion requires all contract, adapter, cache, race, and subprocess tests to pass without network access. Manual Codex and Claude Desktop checks are release acceptance evidence, not substitutes for automated protocol tests.

## Out of Scope

- Streamable HTTP, remote hosting, daemon mode, multi-user operation, authentication, authorization, or tenancy.
- MCP resources, prompts, elicitation, sampling, roots, logging capabilities, or client-specific protocol extensions.
- Natural-language interpretation inside the server.
- Fuzzy-name lookup and autocomplete tools.
- Batch, collection, or deck-list resolution.
- Whole-deck legality, deck-construction validation, synergy analysis, weakness analysis, similarity inference, scoring, substitutions, recommendations, or strategic advice.
- Prices, vendors, purchase links, or any market-data freshness contract.
- Downloading, caching, proxying, embedding, or transforming card images.
- Scryfall bulk-data imports, a full local card database, or offline-first operation beyond stale response fallback.
- Caller-controlled cache refresh, cache administration tools, or a configuration file.
- Additional upstream card-data providers.
- Creating implementation tickets; this document is the canonical feature spec from which tickets may later be decomposed.

## Further Notes

- The repository is currently an empty Go module with domain documentation and no implementation or testing prior art.
- The official primary-source research was verified on 2026-11-16. It records MCP revision `2026-07-28`, official Go SDK v1.8.0, Scryfall's required headers, endpoint-specific rate limits, and 24-hour caching guidance. Reverify time-sensitive dependency versions and service limits immediately before dependency installation.
- The five product tools represent four user capabilities because exact lookup is deliberately split into Oracle-card and printing tools.
- The adapter is replaceable for testability and future source substitution, not an invitation to generalize prematurely. Implement only the domain operations required by the five tools.
- The cache decorates the card-data adapter rather than the MCP layer. This keeps freshness policy consistent across tools and keeps transport tests independent of disk and HTTP details.
- The cursor format is internal and may evolve by version. Invalid or obsolete cursors fail cleanly rather than being guessed or silently restarted.
- Manual release verification should use representative calls for search, Oracle-card lookup, printing lookup, rulings, legality, empty results, exact misses, and stale fallback in both named clients.
- Related decisions are recorded in the glossary and ADRs covering the card-facts boundary, live Scryfall with persistent caching, native Scryfall query syntax, and server-owned public schemas.
