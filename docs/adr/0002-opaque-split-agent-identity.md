# 0002. Agent identity is opaque, and split from the live instance

- **Status:** accepted
- **Date:** 2026-08-04

## Context

Orc has to address a specific running agent — to read its lifecycle, prompt it,
wait on it, or restore it — across session restarts, machine restarts, and
provider resumption.

Every convenient identifier fails at that. A ticket, worker, stage, or window
name is not unique across relaunches. A pane ID identifies a location, not a
process: a live pane can acquire a different process at any time, so session and
window membership prove nothing about what is running there. A PID dies with the
process. A provider session ID belongs to the provider, can be resumed into a
different process, and is absent for agents that never registered one.

The failure this prevents is specific and bad: sending a prompt to a pane that
has been recycled means typing into whatever now occupies it.

## Decision

Every Orc-launched agent gets an **opaque durable agent ID**, derived from
nothing. Every launch of that agent gets an **opaque live instance ID**. Both
are recorded in `STATE.yaml` beside — never merged into — the backend-neutral
target, and stamped onto the pane as metadata.

Identity and target stay separate concepts.

Before Orc sends input to an agent, it validates the recorded pane *and* agent
ID *and* instance ID together. Any mismatch is a refusal, never a fallback to
another pane.

The split gives a precise vocabulary for what "gone" means:

| Evidence | Meaning |
|---|---|
| No multiplexer server | agent is **offline**, not completed |
| No pane | recorded instance is **orphaned** |
| Pane exists, different instance | agent was **replaced**; the old record cannot receive input |
| Provider session resumable | new **instance**, same durable agent ID |

## Consequences

Restoration can honestly claim continuity of the *agent* while admitting the
*instance* is new. That is what lets parking restore work without pretending
the restored process is the original.

Restoring a multiplexer session never proves the restored process is the old
agent, so restoration waits for the provider hook to re-register before
committing the runtime target. Restoration is therefore not instant, and cannot
be made instant without weakening the guarantee.

Every control path pays a validation round-trip before acting, and every backend
must be able to store and return per-pane metadata.

An agent a user started by hand has no Orc identity and never will. It can be
*observed* and displayed, but it cannot be prompted, waited on, or advanced.
That asymmetry is deliberate: identity is what makes an action safe, so actions
are limited to agents that have one.
