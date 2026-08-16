# 0005. Orc and tmux-attention are layered, and neither absorbs the other

- **Status:** accepted
- **Date:** 2026-08-16

## Context

Orc and the `tmux-attention` plugin both describe what an agent is doing, and
both write into the `@agent_*` tmux option namespace. That overlap invites an
obvious question — why two? — and it keeps coming back, because from any single
angle one of them looks redundant.

It is not a hypothetical overlap. The two disagreed on the timestamp field for
long enough that Orc read every plugin-set marker with an age of zero, and
Orc-set markers never appeared in `tmux-fzf-jump`'s attention picker at all.

Two merges are available, and both look reasonable until costed.

## Decision

Keep both. They occupy opposite ends of a trust/coverage trade, and neither
property survives a merge.

| | `tmux-attention` | Orc |
|---|---|---|
| Coverage | every pane, always | panes Orc launched |
| Verification | none — anything may write | pane identity is checked |
| Role | display: glyphs, pickers, status | actions: waits, parking, notification |

Orc **consumes** the plugin's schema for display and never treats it as
authoritative; [ADR-0004](0004-authoritative-sources-are-named.md) names the
sources that may drive actions, and the plugin is not among them.

### Why tmux-attention cannot absorb Orc's role

Orc's notification fires inside the hook process at the moment of transition:
it loads `STATE.yaml`, resolves the ticket, and runs the `notify` command from
`orc.yaml`. Orc has no daemon, so with no hook nothing is running to notice a
transition — `orc watch` is a foreground TUI, not a service.

The plugin could grow a configurable on-change command and dispatch that
notification. But routing a core Orc feature through a tmux plugin contradicts
[ADR-0001](0001-tmux-is-the-default-backend.md): Herdr is a supported backend
with its own native agent status, and it has no tmux options at all. Orc would
need a per-backend notification path, which is the coupling that ADR avoids.

Keeping Orc's hook as a fallback does not help. That is a second path, not one
fewer.

### Why Orc cannot absorb tmux-attention's role

Orc's hooks refuse to write for a pane Orc did not launch:

```go
if fields[2] != event.AgentID || fields[3] != event.AgentInstance {
    return ..., fmt.Errorf("tmux pane %s hosts a different agent instance", pane)
}
```

That check is what makes Orc's signal registered rather than asserted. Removing
it to cover ad-hoc panes would delete the property ADR-0004 depends on; keeping
it and adding an unverified tier alongside is rebuilding the plugin inside Orc,
with the trust boundary moved somewhere no test can see it crossed.

It also inverts a dependency. A `prefix`-bound `M-J` picker answers "which pane
needs me" with nothing installed; `orc focus` answers it only with a configured
`ORC_WORKSPACE`. The layer used constantly should not require the layer used
occasionally.

## Consequences

The cost is a cross-repo schema contract. The plugin owns the option names;
`docs/watch.md` records them and Orc's parser is tested against them. That seam
is where the drift happened and where it will happen again, so it is documented
rather than assumed.

Orc stays useful with no plugin installed, and the plugin stays useful with no
Orc installed. Neither degrades the other's core loop.

Some duplication is accepted and is not a defect: `orc focus` and a pane picker
answer a similar question over different scopes. That is a UX question about
which to reach for, not an argument that one layer should own both.

The question closed here is "should one absorb the other." Narrower questions
stay open — whether the plugin should verify marker ownership, and whether Orc
should read the plugin's active-turn signal — and neither requires a merge.
