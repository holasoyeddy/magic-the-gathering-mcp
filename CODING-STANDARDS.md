# Coding Standards

These standards guide implementation and review. Optimize for code that a human maintainer can understand, verify, and change with minimal context.

## Readability first

- Prefer straightforward code whose behavior is visible from top to bottom.
- Keep the happy path visually dominant. Move validation, retries, cleanup, and error translation behind clearly named functions when they obscure the main operation.
- Use early returns to keep nesting shallow.
- Make policy explicit in names and types. Avoid boolean parameters, magic strings, and generic helpers that hide meaningful distinctions.
- Prefer a small amount of obvious duplication over an abstraction that makes callers harder to understand. Extract shared behavior when it prevents contract drift or gives the behavior one clear home.
- Optimize for locality: understanding one card-fact operation should require opening as few files as practical.

## Functions

- Give each function one coherent job and one reason to change.
- Prefer named functions over long anonymous functions.
- Use an anonymous function only when it is a small, comfortable one-line expression used once at the call site.
- Extract an anonymous function into a named function when it is used more than once, spans multiple lines, contains branching or error handling, or interrupts the surrounding control flow.
- Name functions after the domain behavior they provide, not the implementation technique they happen to use.
- Keep parameter lists focused. When the same values travel together or jointly enforce an invariant, introduce a domain type.
- Every exported function must have a Go documentation comment that begins with the function name and explains its contract. Document important invariants, side effects, error behavior, ordering, cancellation, and concurrency guarantees when they are not obvious from the signature.

## Modules and seams

- Keep MCP transport, card-fact rules, Scryfall translation, cache policy, and process configuration in distinct modules with clear ownership.
- Design deep modules: callers should learn a small interface while the module owns the detailed behavior behind it.
- Place behavior where its governing data and invariants live.
- Accept dependencies at real seams. Introduce an interface when at least two adapters need to vary, including a production adapter and a meaningful test adapter.
- Keep upstream Scryfall types and MCP protocol types at their respective edges. Domain code uses project vocabulary from `GLOSSARY.md`.
- A change to one card-fact capability should have one obvious implementation home. Scattered synchronized switches or conditionals are a signal to centralize the policy.

## Names and domain types

- Use the exact domain language from `GLOSSARY.md`: Oracle card, printing, card fact, card search, exact card lookup, card legality, and ruling.
- Choose names that reveal purpose without requiring the reader to inspect the implementation.
- Represent behavior-controlling concepts with named types or typed constants. Examples include selectors, cache operations, provenance, endpoint classes, result modes, and failure codes.
- Keep abbreviations conventional and unambiguous. Prefer complete domain names when an abbreviation would make a reader pause.

## Control flow and errors

- Keep resource ownership visible. The code that opens a response body, file, timer, or session must make its cleanup responsibility obvious.
- Handle or intentionally discard errors at the point where the policy is known. Mark intentional discards explicitly.
- Wrap errors with operation context while preserving the underlying error for `errors.Is` and `errors.As`.
- Translate errors once at the seam that owns the public contract. Do not leak upstream payloads or infrastructure details into MCP results.
- Preserve cancellation through every blocking operation. Cancellation paths must release reservations and resources promptly.

## Comments and documentation

- Write comments for policy, invariants, ownership, and non-obvious tradeoffs.
- Keep comments synchronized with behavior and place them beside the code they explain.
- Prefer a descriptive name over a comment that merely restates the code.
- Record hard-to-reverse architectural or product decisions in an ADR instead of relying on source comments.

## Tests

- Test client-visible behavior through the narrowest stable interface, with MCP contracts as the primary product seam.
- Name tests after the behavior and outcome they prove.
- Keep setup noise behind named helpers so each test reads as arrange, act, and assert.
- Prefer table-driven tests when cases share one behavior and setup. Prefer separate tests when combining them would hide distinct policies or failure modes.
- Use anonymous test callbacks only when they remain small and immediately readable. Repeated or multi-line behavior belongs in a named helper or test adapter.
- Assert outcomes and durable contracts rather than private call graphs or incidental implementation details.

## Review completion

A review is complete only when every changed path has been checked for:

- a readable happy path and shallow control flow;
- appropriately sized, named functions;
- documentation comments on every exported function;
- domain names and types that preserve project vocabulary;
- clear module ownership and resource ownership;
- errors and cancellation handled at the correct seam;
- tests that describe observable behavior;
- abstractions that improve locality or prevent contract drift rather than merely reduce line count.
