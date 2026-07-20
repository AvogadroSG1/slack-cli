---
status: accepted
date: 2026-07-20
deciders:
  - Peter O'Connor
consulted:
  - Codex specification reviewer
---

# ADR-0001: Evolve `thread-read` into an exhaustive semantic command

## Context and problem statement

The existing `thread-read` command retrieves one `conversations.replies` page and discards message reactions. It requires flags even when the caller has copied a Slack permalink, and it can present a partial page as if it were a complete thread.

Callers need one stable semantic command that accepts their normal pasted permalink, starts at the parent message, retrieves every reply in chronological order, preserves useful narrowing controls, and represents reactions consistently for humans and automation.

## Decision drivers

- A copied Slack permalink MUST be directly usable.
- Complete thread retrieval MUST be the default.
- Existing invocations and JSON array topology MUST remain compatible.
- Human output MUST remain concise.
- JSON MUST preserve exact Slack message identity.
- Pagination limits and incomplete results MUST be unambiguous.
- The retrieval loop MUST be isolated and testable without live Slack access.
- Reactor identities MUST NOT be implied to be complete when Slack does not guarantee completeness.

## Considered options

1. Evolve the existing `thread-read` command.
2. Add a new `thread-retrieve` command.
3. Add a general `read` command that infers message and thread behavior.

## Decision outcome

The existing `thread-read` command will be evolved because it already owns the semantic thread-reading use case and provides the established human and JSON output contracts.

The command will accept a positional permalink while preserving `--url` and `--channel` plus `--ts`. It will retrieve all cursor pages by default, treat `--limit` as API page size and `--max-results` as the total cap, disclose capped results, and include reaction name/count summaries on every message.

JSON will remain a top-level array. Each JSON message will add exact `slack_ts` and an always-present `reactions` array. Human output will add one indented reaction line only when reactions exist.

### Consequences

- Good, because callers can paste the permalink they already copy from Slack.
- Good, because the default result is complete rather than silently limited to one API page.
- Good, because existing commands and output topology remain compatible.
- Good, because exact Slack timestamps preserve message identity for automation.
- Good, because reaction counts are useful without overstating the completeness of reactor lists.
- Good, because an injectable pagination boundary enables deterministic BDD coverage.
- Bad, because exhaustive retrieval can make additional API calls and encounter rate limits.
- Bad, because the command needs dedicated pagination behavior instead of reusing the existing generic helper unchanged.
- Bad, because JSON consumers that reject unknown fields may need to permit the additive `slack_ts` and `reactions` fields.
- Bad, because capped results require callers to observe stderr for the resumable cursor while stdout remains the compatible array.

## Confirmation

The decision is confirmed when:

- BDD scenarios cover positional and reply permalinks, legacy inputs, multi-page retrieval, narrowing, caps, retries, ordering, deduplication, reactions, metadata, and error paths.
- `make test` and `make lint` pass.
- README, CLI skill guidance, help text, contributor documentation, the design specification, and this ADR agree.
- A reviewer confirms that `message-read` and the generic `conversations replies` command retain their existing contracts.

## Pros and cons of the options

### Evolve `thread-read`

- Good, because it strengthens the command users already discover for this task.
- Good, because compatibility can be additive.
- Bad, because the implementation must carefully separate thread-specific formatting from shared `message-read` formatting.

### Add `thread-retrieve`

- Good, because it could begin with a clean interface.
- Bad, because two commands would compete for the same semantic use case.
- Bad, because existing users would face an unnecessary migration and deprecation path.

### Add a general `read` command

- Good, because a single entry point could eventually cover standalone messages and threads.
- Bad, because inference rules introduce ambiguity and expand the scope beyond thread retrieval.
- Bad, because it does not solve the focused requirement more clearly than evolving `thread-read`.

## More information

- [Approved design specification](../superpowers/specs/2026-07-20-thread-read-reactions-design.md)
- [Slack `conversations.replies` reference](https://docs.slack.dev/reference/methods/conversations.replies/)
- [Slack message reaction behavior](https://docs.slack.dev/reference/events/message/)
- [MADR template primer](https://www.ozimmer.ch/practices/2022/11/22/MADRTemplatePrimer.html)

*Authored By Peter O'Connor with Assistance from Claude Code (GPT-5) · 2026-07-20 · slack-cli exhaustive thread-read ADR*
