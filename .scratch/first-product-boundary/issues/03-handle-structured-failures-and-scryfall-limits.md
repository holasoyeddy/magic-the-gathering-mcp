# 03: Handle structured failures and Scryfall limits

**What to build:** Make exact card lookup resilient and predictable by mapping expected failures to stable MCP-facing errors and making the live Scryfall adapter a responsible, cancellation-aware API consumer.

**Blocked by:** 01: Start the server with exact Oracle-card lookup.

**Status:** resolved

- [x] Expected failures use the stable codes `not_found`, `invalid_query`, `invalid_identifier`, `rate_limited`, and `upstream_unavailable` with concise messages and no raw upstream body.
- [x] Exact lookup misses return `not_found`; unexpected failures return a generic internal error while diagnostic detail remains on standard error.
- [x] Every Scryfall request sends an accurate application-specific `User-Agent` and JSON-compatible `Accept` header.
- [x] One shared limiter registry enforces the documented endpoint classes, including the stricter interval for named lookup and search.
- [x] HTTP 429 handling honors `Retry-After`, and a request unable to enter the limiter within its bounded execution window returns `rate_limited`.
- [x] Transient failures receive no more than two bounded retries; invalid requests, definitive misses, and other non-transient client errors are not retried.
- [x] MCP cancellation propagates through adapter calls, limiter waits, retries, and HTTP requests.
- [x] Redirects or requests to destinations outside the allowed Scryfall hosts are rejected without forwarding identifying headers.
- [x] MCP contract tests verify client-visible error mapping and cancellation with the fake adapter.
- [x] Narrow scripted-transport tests verify rate spacing, `Retry-After`, retry ceilings, non-retryable responses, cancellation during waits, and the network boundary.

## Answer

Implemented stable MCP-facing failure translation and generic internal-error handling with diagnostics confined to stderr. The Scryfall adapter now uses a shared cancellation-aware limiter, sends the required headers, honors `Retry-After`, retries transient failures at most twice, avoids retrying definitive client failures, and rejects cross-host redirects before identifying headers can be forwarded. Contract and scripted-transport tests cover the client-visible and network behaviors.

