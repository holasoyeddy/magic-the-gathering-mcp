# 02: Add exact printing lookup

**What to build:** Add `get_printing` as a complete MCP capability that resolves one printing by Scryfall ID or by set code and collector number, returning edition-specific facts without duplicating the complete Oracle-card payload.

**Blocked by:** 01: Start the server with exact Oracle-card lookup.

**Status:** resolved

- [x] Tool discovery deterministically includes `get_printing` alongside `get_oracle_card`.
- [x] The tool accepts exactly one complete selector: Scryfall ID, or set code plus collector number.
- [x] Missing, partial, malformed, or conflicting selectors fail before upstream access.
- [x] Collector number is treated as text and set-code input is case-insensitive with a normalized result.
- [x] A successful result includes Scryfall ID, Oracle ID when available, language, localized printed fields when available, set identity, collector number, rarity, release date, artist, finishes, frame treatment, and image URLs when applicable.
- [x] Multi-face printings preserve face order and face-specific fields.
- [x] Results contain Scryfall-hosted image URLs only; the server does not fetch, cache, proxy, embed, or transform image content.
- [x] Results reference Oracle identity without duplicating the complete Oracle-card result.
- [x] MCP contract tests cover Scryfall-ID lookup, set-and-collector lookup, a non-English printing, a multi-face printing, optional fields, and invalid selectors through the fake adapter.
- [x] Narrow Scryfall adapter tests verify printing payload translation without exposing upstream field names to the MCP layer.

## Answer

Implemented `get_printing` as the second deterministic MCP tool. It validates either a Scryfall UUID or a complete set-code/collector-number selector before adapter access, returns a curated printing schema with provenance, preserves localized and multi-face data, and limits image URLs to Scryfall's HTTPS image host. Contract tests exercise both selectors and invalid inputs through MCP; narrow adapter tests cover payload translation and optional fields.

