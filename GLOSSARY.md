# Magic: The Gathering MCP

The shared language for a client-portable MCP server that exposes authoritative Magic: The Gathering card information.

## Language

**Oracle card**:
The shared game object represented by all printings with the same Oracle identity.
_Avoid_: Printing, edition, card version

**Printing**:
A set-specific publication of an Oracle card, identified unambiguously by a Scryfall ID or by set code and collector number.
_Avoid_: Oracle card

**Card fact**:
Authoritative information about a card, printing, ruling, or format legality that can be traced to the upstream card-data source.
_Avoid_: Card intelligence, card advice

**Deck intelligence**:
Derived evaluation or advice about a collection of cards, such as synergy analysis, weakness detection, substitutions, or strategic recommendations.
_Avoid_: Card search, card facts

**Card search**:
Discovery of cards or printings that match user-supplied criteria, returning only upstream card facts.
_Avoid_: Deck search, recommendations

**Exact card lookup**:
Resolution of an unambiguous identifier to one authoritative result. Oracle-card lookup and printing lookup are separate operations.
_Avoid_: Card search

**Card legality**:
The upstream format-legality status of one Oracle card, independent of any deck that contains it.
_Avoid_: Deck legality, deck validation

**Ruling**:
An official dated clarification of how an Oracle card functions, shared by all of its printings.
_Avoid_: Strategy, recommendation, printing ruling

**MCP client**:
Any standards-compliant application that connects to and invokes the server; Codex and Claude Desktop are compatibility targets, not privileged clients.
_Avoid_: Codex client, Claude client
