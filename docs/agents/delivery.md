# Delivery: One Ticket, One Pull Request

The default delivery unit is one implementation ticket. Each ticket gets its own branch, review, and pull request so a human can understand and approve the change independently.

## Before implementation

- Start only tickets on the frontier: every blocking ticket must already be merged into the target branch.
- Confirm the ticket delivers one complete, demonstrable vertical slice.
- Split the ticket before coding when its expected diff cannot be reviewed comfortably in approximately 15–30 minutes, introduces more than one major policy, or requires unrelated modules to change.
- For a wide mechanical refactor, use expand–migrate–contract tickets. Each stage remains independently mergeable and gets its own pull request.

## Per-ticket flow

1. Update the target branch and create a fresh ticket branch from it.
2. Implement only that ticket with `/implement`, including its tests and documentation.
3. Run `make check` once at handoff, then run `/code-review` against the target branch.
4. Fix every blocking review finding on the ticket branch.
5. Open one pull request whose scope and title match the ticket.
6. Merge the pull request after human review and required checks pass.
7. Resolve the ticket according to `docs/agents/issue-tracker.md` only after the pull request merges.
8. Recompute the frontier from merged tickets before starting dependent work.

The ticket is delivery-complete only when its pull request is merged and its tracker file is resolved.

## Verification tiers

Match verification cost to the current phase. Advance to a more expensive tier only at its named gate.

### Development: targeted feedback

Run the narrowest test that can turn red for the behavior being changed:

```sh
make test PACKAGES=./internal/scryfall GO_TEST_FLAGS='-run ^TestName$'
```

Repeat targeted tests during red-green-refactor work. Expand to the affected package when the change crosses several behaviors.

### Ticket handoff and pull request: quick guardrail

Run once after targeted tests pass:

```sh
make check
```

This is the required pull-request check. It runs lint, vet, the offline test suite, and a build without race or fuzz work.

### Merge: integration-focused tests

After a clean merge, run tests for the affected packages. Run the complete offline suite when conflict resolution or a shared seam affects multiple packages:

```sh
make test
```

Merger agents rely on the ticket's successful `make check`; they do not repeat race and fuzz verification.

### Main branch and release: full guardrail

Run once for the integrated candidate:

```sh
make check-full
```

This adds the race detector and bounded fuzz smoke tests. CI runs the equivalent expensive tier after changes reach `main`, not for every pull-request revision. Re-run it after a fix only when that fix changes concurrency, shared infrastructure, process startup, cache behavior, or network behavior.

## Parallel work

Independent frontier tickets may run in parallel. Each starts from the same reviewed target tip and remains a separate branch and pull request. If one pull request changes a shared seam, update and re-verify the others before merging them.

Dependent tickets wait for their blockers to merge. An implemented branch or open pull request does not satisfy a blocking edge.

## Pull-request scope

A ticket pull request contains:

- the behavior and acceptance criteria of one ticket;
- tests that prove that behavior;
- documentation required by that behavior;
- small preparatory refactors required to make the change safely.

Move independent cleanup, broad architecture changes, and unrelated formatting into separate tickets and pull requests.

## Exceptions

Use `/implement-spec` only when the user explicitly chooses integration throughput over incremental review and accepts one spec-sized integration branch and pull request.

Use stacked pull requests only when the user explicitly chooses them. Each pull request targets its immediate predecessor, remains independently reviewable, and is retargeted or rebased as predecessors merge.
