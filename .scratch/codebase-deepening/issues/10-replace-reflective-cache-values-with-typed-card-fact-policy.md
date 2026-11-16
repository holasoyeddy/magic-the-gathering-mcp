# 10: Replace reflective cache values with typed card-fact policy

**What to build:** Make cache operation identity, provenance, serialization, and stale marking compiler-visible so adding or changing a card fact no longer depends on reflection, field-name conventions, or synchronized runtime type switches.

**Blocked by:** 03: Make the card-fact adapter capability set truthful.

**Status:** ready-for-agent

- [ ] Cached card facts expose provenance and stale behavior through typed domain contracts rather than reflective field lookup.
- [ ] Each cache operation has one typed identity that owns keying, serialization, decoding, and provenance behavior.
- [ ] Adding a card-fact capability does not require synchronized edits across unrelated switches or field-name conventions.
- [ ] The cache remains an adapter at the live-versus-cached seam; cache mechanics do not leak to MCP transport or Scryfall translation.
- [ ] Fresh reuse, stale fallback, original fetch timestamps, single-flight behavior, corruption recovery, schema rejection, and LRU limits remain unchanged.
- [ ] Persistence compatibility is preserved or an explicit version transition rejects old records safely.
- [ ] Cache behavior is verified through its adapter interface with focused persistence coverage.
- [ ] `make check` passes.
