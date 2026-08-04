# Architecture decision records

Short records of decisions that shape Orc and that keep getting reopened.

`CONTEXT.md` defines Orc's vocabulary and states the durable decisions in one
line each. When a decision needs its reasoning, its alternatives, or the
consequences a future contributor would otherwise have to rediscover, it gets a
record here and `CONTEXT.md` keeps its one-line version.

Write a record when a decision is:

- **Load-bearing** — code, workflow policy, or the workspace scaffold depends
  on it holding.
- **Repeatedly reopened** — the same question keeps coming back because the
  reasoning lives only in a commit message or someone's memory.
- **Non-obvious** — the alternative looks better until you know what it costs.

Do not write one for decisions the code already documents, or for anything a
`CONTEXT.md` line fully covers.

## Format

One file per decision, `NNNN-kebab-case-title.md`, numbered in order. Keep it
short — a record nobody reads is not a record.

```markdown
# NNNN. Title

- **Status:** proposed | accepted | superseded by [NNNN](NNNN-....md)
- **Date:** YYYY-MM-DD

## Context

What forced the decision. The constraint, not the history.

## Decision

What was decided, in the present tense.

## Consequences

What this makes easy, what it makes hard, and what it rules out.
```

Records are immutable once accepted. To change a decision, add a new record and
mark the old one superseded — the fact that a decision was reconsidered is
usually the most useful thing in the directory.

## Records

| # | Title | Status |
|---|-------|--------|
| [0001](0001-tmux-is-the-default-backend.md) | tmux is the default backend, Herdr is additive | accepted |
| [0002](0002-opaque-split-agent-identity.md) | Agent identity is opaque, and split from the live instance | accepted |
| [0003](0003-ranked-lifecycle-evidence.md) | Lifecycle evidence is ranked, and only registration is authoritative | accepted |
