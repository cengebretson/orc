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
| `W` | Toggle a 32-column `orc watch` rail on the right. |
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

## Split alternatives

`orc watch --tmux-toggle` owns its split pane, marks it with `@orc_watch`, and
closes the same pane when invoked again. To put the rail on the bottom instead:

```tmux
bind-key W run-shell 'orc --workspace "$ORC_WORKSPACE" watch --tmux-toggle --tmux-layout bottom --tmux-size 25%'
```

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
