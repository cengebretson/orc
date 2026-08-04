# 0001. tmux is the default backend, Herdr is additive

- **Status:** accepted
- **Date:** 2026-08-03

## Context

Orc runs agents inside a terminal multiplexer. Two candidates exist in the
codebase: tmux, and Herdr, which offers richer native primitives — opaque exact
targets, native agent lifecycle, worktree create/open, task-cell layouts, and
sidebar metadata. Herdr can express things tmux can only approximate, so there
is standing pressure to treat it as the primary backend and let tmux degrade.

The constraint is where Orc has to run. Orc's value is carrying feature work
across sessions and machines, including over SSH onto a box where the only
thing installed is tmux. A backend that is not present everywhere Orc is used
cannot be the one the workflow depends on.

## Decision

tmux is the default backend and the portability floor. Herdr is an optional,
additive backend selected per command with `--mux herdr` or by a runtime
recorded in `STATE.yaml`.

Both sit behind `mux.Backend`. Backend-specific capability is reached through
narrow optional interfaces a backend may implement (for example
`mux.AgentAcknowledgeBackend`, `mux.FallbackObservationBackend`), never by
branching on a backend name in workflow code.

Orc owns workflow state and durable identity. Backends own terminal
composition and exact target operations, and nothing else.

## Consequences

A normal Orc workspace stays fully usable with only tmux installed, and every
feature reachable without Herdr keeps working when Herdr is absent.

Herdr-only capability has to degrade rather than disappear: where tmux cannot
match a native primitive, Orc falls back to a conservative approximation
(installed provider hooks for lifecycle, bounded title and screen inference for
presentation only) and marks the difference in its source fields rather than
pretending the evidence is equivalent.

The cost is a seam. Every live-runtime feature is written twice, and
`mux.Backend` accumulates optional interfaces as backends diverge. That is
accepted deliberately: it is the price of Orc running wherever tmux runs.

This rules out making Herdr required, letting Herdr own workflow state, and
building terminal compositing or a grid of live drivable panes into Orc —
Herdr owns that layer.
