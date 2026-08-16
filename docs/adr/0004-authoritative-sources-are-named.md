# 0004. Authoritative sources are named, not inferred by exclusion

- **Status:** accepted
- **Date:** 2026-08-16

## Context

[ADR-0003](0003-ranked-lifecycle-evidence.md) established that only
registration is authoritative and that inference is presentation-only. The code
implemented that by *excluding* the sources known not to qualify:

```go
if item.Lifecycle != "" && item.LifecycleSource != "launch" &&
   item.LifecycleSource != "screen" && item.LifecycleSource != "title" {
```

That reads as the decision only while Orc is the sole writer of a source value.
It is not. `@agent_attention` and its pane-scoped equivalents are a shared
namespace: the `tmux-attention` plugin writes markers from Claude and Codex
hooks, and its CLI takes a free-text `--source`, so the recorded value is
whatever the caller passed. A pane can carry `source=claude`, `source=orc`, or
anything a shell script chose.

Under exclusion, every one of those qualified as authoritative — they are not
`launch`, `screen`, or `title`. The same shape in `parkingAttention` meant a
marker Orc never registered could wake parked work.

The failure is quiet and asymmetric. Adding a source anywhere — a new backend, a
plugin release, a user's script — silently granted it the power to advance a
stage or satisfy `orc ctl agent wait`, with no change to Orc and nothing to
review.

## Decision

Authority is a named list, in one predicate:

```go
func IsRegisteredSource(source string) bool {
    return source == SourceHook || source == SourceNative
}
```

`hook` is Orc's provider lifecycle hooks; `native` is a backend's own agent API.
Both are channels Orc can verify: the hook path checks `@orc_agent_id` and
`@orc_agent_instance` against the pane before it writes, so it only reports for
agents Orc launched.

Everything else — inference, Orc's own `launch` reset, and any source arriving
from outside Orc — is presentation. It renders, it sorts, it never acts.

A self-asserted string cannot be a trust boundary. `--source claude` is a label
the caller chose, not evidence that Claude reported anything, so no amount of
matching on it can make a marker authoritative.

## Consequences

New sources default to safe. A backend, plugin, or script that starts writing
markers shows up in the display and cannot advance a stage until someone adds it
to the predicate deliberately.

`tmux-attention` markers no longer wake parked work. That is a behavior change:
previously any marker whose source was not `screen` or `title` could. Orc still
reads those markers and still shows them — what it stops doing is acting on a
report it cannot confirm.

The cost is that Orc ignores a signal that is usually correct. A hook-set
`blocked` marker is nearly always a real blocked agent. But "nearly always" is
the property ADR-0003 already rejected for inference, and the reason is the
same: the interface must distinguish what can be acted on from what merely looks
right.

Nothing here changes what Orc reads. Consuming `tmux-attention` for display
stays supported and is how a pane Orc never launched still appears in a live
view.
