# Tmux integration

Orc does not install or change tmux configuration. The examples here are
optional, user-owned shortcuts for the live-session commands.

## Copyable defaults

[`docs/tmux.conf`](tmux.conf) contains a syntax-checked starting point. Copy the
bindings you want into `~/.tmux.conf`, replace the workspace placeholder with an
absolute path, then reload tmux:

```sh
tmux source-file ~/.tmux.conf
```

The defaults use these prefix bindings:

| Key | Action |
|-----|--------|
| `W` | Toggle a 64-column managed Orc rail on the right. |
| `C` | Collapse or restore the managed rail without restarting it. |
| `P` | Open the wide watch dashboard in a 90% by 80% popup. |
| `S` | Show `orc sessions --all` in a pageable popup. |
| `R` | Open the interactive `orc sessions resume` picker in a popup. |
| `F` | Run `orc focus` and jump to the next session needing attention. |

All bindings run after the normal tmux prefix. Change the keys freely if they
conflict with existing configuration.

## Workspace selection

The example sets one explicit workspace for the tmux server:

```tmux
set-environment -g ORC_WORKSPACE "/absolute/path/to/orc-workspace"
```

An absolute path keeps bindings reliable when the active pane is inside another
repository or directory. For multiple Orc workspaces, set the variable on each
tmux session instead of globally:

```sh
tmux set-environment -t my-tmux-session ORC_WORKSPACE /absolute/path/to/workspace
```

If every binding runs from inside the Orc workspace, you may omit
`--workspace "$ORC_WORKSPACE"`; Orc searches upward from the command's working
directory for `orc.yaml`.

## Managed rail

`orc rail open|close|toggle` manages one Orc-owned `orc watch` pane in the
current tmux window. Opening reuses an existing rail, preserves focus, and
stamps `@orc_rail=1` plus `@orc_role=rail`; closing refuses to kill a pane when
ownership is absent or ambiguous. The pane remains a normal mouse-resizable
tmux pane.

Collapse resizes the live pane instead of restarting it:

```sh
orc rail collapse
orc rail expand
orc rail toggle-collapsed
```

Orc remembers the current mouse-adjusted size before collapsing, uses five
columns as the compact floor, and displays `»` on the rail's last row. Override
the initial 64-column size or choose a bottom rail when opening:

```tmux
bind-key W run-shell 'orc --workspace "$ORC_WORKSPACE" rail toggle --layout bottom --size 18'
```

Rows display authoritative lifecycle age. A continuously `working` agent is
marked `stuck` after 15 minutes by default; configure another positive Go
duration with `settings.rail.stuck_after` (for example `30m`). Explicit Orc
attach, focus, prompt, and Live-rail actions acknowledge the exact agent
sequence. State reads and captures never acknowledge it.

Authoritative hook-published `blocked` and unseen `done` transitions use
`settings.notify`; duplicate events and acknowledgements do not notify again.

When hooks are missing, the rail can use conservative title and bounded-screen
rules without promoting terminal text to control state. See
[Tmux fallback detection](agent-detection.md) for precedence, local versioned
overrides, debounce behavior, and safety boundaries.

`orc sessions resume` continues the selected provider in the foreground. The
popup therefore remains open for the resumed session's lifetime. Use a normal
pane when that is more convenient:

```tmux
bind-key R split-window -v -l 40% -c "#{pane_current_path}" 'exec orc --workspace "$ORC_WORKSPACE" sessions resume'
```

## Agent prompting

With Orc's Codex or Claude lifecycle hooks installed, tmux implements the same
structured state, prompt, wait, and watch commands as the Herdr backend. Prompt
delivery revalidates the recorded pane and agent instance, loads at most 64 KiB
of UTF-8 text into a private tmux buffer over stdin, requires bracketed paste,
and sends Enter as an encoded key. Prompt text is never interpolated into a
shell command or command-line argument.

In `orc watch`, press `s`, compose the message, press `enter` to review, and
press `y` to confirm. For automation, use:

```sh
orc ctl agent prompt --ticket ORC-9 "Review this diff" --wait --timeout 120s
```

When `--wait` is used, tmux must observe a new authoritative lifecycle sequence
within the startup grace period. Otherwise Orc returns
`agent_prompt_stalled`; replacement, exit, cancellation, and timeout remain
distinct structured errors.

## Verification

Reload the configuration and inspect the bindings:

```sh
tmux source-file ~/.tmux.conf
tmux list-keys | rg 'orc --workspace'
tmux show-environment -g ORC_WORKSPACE
```

`source-file` returning successfully verifies tmux syntax. It does not validate
the placeholder path, so run one binding after setting the real workspace.
