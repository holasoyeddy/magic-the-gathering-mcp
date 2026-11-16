# Domain Docs

How the engineering skills should consume this repo’s domain documentation when exploring the codebase.

## Before exploring, read these

- `GLOSSARY.md` at the repository root
- Relevant ADRs under `docs/adr/`

If these files do not exist, proceed silently. The domain-modeling skill creates them lazily when terminology or decisions are resolved.

## File structure

This is a single-context repository:

```text
/
├── GLOSSARY.md
├── docs/adr/
└── src/
```

## Use the glossary’s vocabulary

When output names a domain concept, use the term defined in `GLOSSARY.md`. Avoid synonyms the glossary explicitly rejects.

If a needed concept is absent, reconsider whether it is project terminology or note the gap for domain modeling.

## Flag ADR conflicts

Explicitly surface output that contradicts an existing ADR rather than silently overriding the recorded decision.
