# Tmux fallback detection

Orc prefers provider hooks for tmux lifecycle state. When an Orc-managed Codex
or Claude pane has no hook-backed state, the Live rail may use terminal titles
and then a bounded bottom-of-screen capture as presentation-only fallback.

Precedence is strict:

1. Hook lifecycle and attention metadata.
2. OSC pane-title rules.
3. Bounded screen rules.
4. Explicit `unknown`.

Fallback state is exposed as `observed_lifecycle` with `observation_source`
equal to `title` or `screen`. It never replaces hook-owned `lifecycle`, satisfies
`orc ctl agent wait`, sends completion notifications, advances a workflow, or
wakes automatic parking. `internal/sessionlist` uses the exact recorded pane;
the window attention rollup remains a presentation convenience for the
Workspace table.

## Versioned manifests

The binary embeds strict YAML manifests for Codex and Claude under rule version
1. Rules have explicit priority, source, lifecycle, optional attention,
regular-expression pattern, and—when inspecting screen text—a bounded
`region_lines` value no greater than 80. Title rules outrank screen rules.

A workspace can completely replace an engine manifest locally:

```text
agent-detection/v1/codex.yaml
agent-detection/v1/claude.yaml
```

The file must declare the current version and matching engine. Unknown fields,
duplicate IDs, invalid regular expressions, unsupported states, and unbounded
regions fail the Live refresh visibly. Orc never downloads detection rules.

Example:

```yaml
version: 1
engine: codex
rules:
  - id: local-approval
    source: screen
    lifecycle: blocked
    attention: blocked
    priority: 950
    region_lines: 12
    pattern: '(?i)approve local operation'
```

Working-to-idle title transitions require two consecutive observations. A new
agent instance clears inherited observed state before startup. Leading spinner
glyphs are removed from display titles so animation does not make row labels
flicker.
