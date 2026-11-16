# 02: Add exact printing lookup

**What to build:** Add `get_printing` as a complete MCP capability that resolves one printing by Scryfall ID or by set code and collector number, returning edition-specific facts without duplicating the complete Oracle-card payload.

**Blocked by:** 01: Start the server with exact Oracle-card lookup.

**Status:** ready-for-agent

- [ ] Tool discovery deterministically includes `get_printing` alongside `get_oracle_card`.
- [ ] The tool accepts exactly one complete selector: Scryfall ID, or set code plus collector number.
- [ ] Missing, partial, malformed, or conflicting selectors fail before upstream access.
- [ ] Collector number is treated as text and set-code input is case-insensitive with a normalized result.
- [ ] A successful result includes Scryfall ID, Oracle ID when available, language, localized printed fields when available, set identity, collector number, rarity, release date, artist, finishes, frame treatment, and image URLs when applicable.
- [ ] Multi-face printings preserve face order and face-specific fields.
- [ ] Results contain Scryfall-hosted image URLs only; the server does not fetch, cache, proxy, embed, or transform image content.
- [ ] Results reference Oracle identity without duplicating the complete Oracle-card result.
- [ ] MCP contract tests cover Scryfall-ID lookup, set-and-collector lookup, a non-English printing, a multi-face printing, optional fields, and invalid selectors through the fake adapter.
- [ ] Narrow Scryfall adapter tests verify printing payload translation without exposing upstream field names to the MCP layer.

