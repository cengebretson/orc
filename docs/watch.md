# orc watch spec

`orc watch` is a compact tmux-side dashboard for active agent work. It is meant
to run in a narrow vertical split, not replace the full `orc tui`.

## Goals

- Show active sessions and their current state at a glance in a 24-32 column pane.
- Let a user quickly select a ticket and act on its prompt.
- Keep `STATE.yaml` as the durable source of truth.
- Optionally consume `tmux-attention` as a transient backup signal.
- Keep the full `orc tui` as the broad dashboard and detail browser.

## Non-goals

- Do not build a full terminal multiplexer.
- Do not require `tmux-attention`.
- Do not infer complex semantic agent state from terminal output in v1.
- Do not make prompt sending automatic on selection.
- Do not expand the main `orc tui` layout to support this narrow rail.

## Command shape

```sh
orc watch
orc watch PROJ-123
orc watch --interval 2s
orc watch --wide
orc watch --tmux-toggle
orc watch --tmux-toggle --tmux-layout bottom --tmux-size 25%
orc focus
```

`orc watch` defaults to a compact multi-ticket rail. `orc watch PROJ-123`
focuses a single ticket and can show a little more context. `--wide` may render a
table-like session view when the pane is wide enough, with worker shown as a
column in the session list rather than as a separate list. Pressing `enter`
opens the selected story details page in both compact and wide layouts, including
recent story history. `--tmux-toggle` is a helper for tmux keybindings. By
default it opens a 32-column right-side pane; `--tmux-layout bottom` and
`--tmux-size <size>` can override the split.

See [Tmux integration](tmux.md) for copyable watch popup and split bindings,
workspace selection, and syntax verification. Orc never installs those bindings.

Press `/` to filter the rail without changing durable state or session
priority. The shared matcher covers ticket, slug, workflow, stage, worker,
engine, repository, branch, status, and attention state. Enter keeps the active
filter; Escape clears it.

## Narrow rail view

The default view should assume a narrow vertical split. Avoid tables, headers,
long labels, and dense detail.

Example multi-session rail:

```text
ORC

SESSIONS
! PROJ-123
? PROJ-124
x PROJ-125

DETAIL
! blocked
develop
bob-dev
tmux live

Next
fix tests
```

Example focused ticket view:

```text
ORC

PROJ-123
develop
bob-dev

! blocked
tmux live

Next
fix tests
```

The renderer should truncate cleanly to the available width. The primary signal
is the state icon/label; the selected session should show a small details block
below the list so the user does not need to press a key for basic context.
Within that block, keep status on the first line, indent stage/worker/tmux
metadata, and render `Blocker` or `Next` as its own section heading before the
prompt text.

## State model

`orc watch` should be driven primarily by the durable Orc workflow contract.
`STATE.yaml` is authoritative for whether work needs a human, is done, is
pending, or is active. Live runtime checks should only refine that durable view.

The important contract point is that an agent needing human input is not just a
terminal condition. The agent must run:

```sh
orc mark <ticket> pause "<what you need>"
```

before asking the human. That makes `STATE.yaml` show `paused`, and `orc watch`
should display that as blocked/human-needed without requiring any tmux marker.

`orc watch` combines these sources:

- `STATE.yaml`: ticket, slug, workflow, stage, worker, status, next action.
- tmux: session/window exists or stopped.
- `tmux-attention`: optional current window marker from `@agent_attention`.
- Provider telemetry: optional context usage and limit, correlated to the exact
  managed pane without changing durable state.

Initial state mapping and precedence:

| Priority | Source | Value | Watch display |
|----------|--------|-------|---------------|
| 1 | `STATE.yaml` | `paused` | `blocked` |
| 2 | `STATE.yaml` | `done` | `done` |
| 3 | `STATE.yaml` | `pending` | `pending` |
| 4 | tmux | configured session/window missing | `stopped` |
| 5 | `@agent_attention` | `input` | `input` |
| 6 | `@agent_attention` | `blocked` | `blocked` |
| 7 | `@agent_attention` | `review` | `review` |
| 8 | `@agent_attention` | `done` | `done` |
| 9 | fallback | active session, no marker | `active` |

`tmux-attention` must not override stronger durable workflow states such as
`paused`, `done`, or `pending`. It is a backup/live overlay for active work where
the durable state does not already explain what the user should see.

Rows are ordered by urgency, with load errors and blocked work first, followed
by input, review, stopped, ready, pending, active, and done work. Tickets with
the same urgency remain alphabetically ordered.

## Context pressure

The wide session table and selected-session details display live provider
context usage as a percentage. The full TUI uses the same classifier in its
feature table and detail view. Configure the color boundaries in `orc.yaml`:

```yaml
settings:
  context_pressure:
    green: 0
    yellow: 70
    red: 90
```

Values must satisfy `0 <= green < yellow < red <= 100`. When provider telemetry
is matched but the provider does not report its context limit, the display says
`n/a` rather than treating the limit as zero. A dash means no live telemetry was
matched. Context pressure is presentation-only: it does not change priority,
workflow state, stage advancement, or session lifecycle.

## tmux-attention integration

`tmux-attention` can provide a low-level tmux notification primitive:

- CLI states: `input`, `blocked`, `review`, `done`, `clear`.
- tmux window option: `@agent_attention`.
- status rendering through `@tmux_attention_status`.
- Claude/Codex hooks that call the CLI.
- clear-on-view behavior through tmux hooks.

`orc watch` consumes this state when available:

```sh
tmux show-options -w -t <session>:<window> -v @agent_attention
```

`orc` may optionally emit it from workflow transitions, but `STATE.yaml` remains
authoritative:

| orc event | tmux-attention state |
|-----------|----------------------|
| `orc mark <ticket> pause` | `blocked` |
| `orc mark <ticket> next` | `done` or `review` |
| `orc mark <ticket> done` | `done` |
| `orc mark <ticket> start` | `clear` |
| `orc mark <ticket> resume` | `clear` |

This integration is optional and is a no-op when `tmux-attention` is not installed.

When `STATE.yaml` says `paused`, watch should show `blocked` even if
`@agent_attention` is empty or stale. When `STATE.yaml` says `active`,
`@agent_attention=input|blocked|review|done` can refine the live display.

Important constraint: `tmux-attention` is currently window-scoped. This fits
`orc` because workflow stages map to tmux windows. If multiple agent panes share
one tmux window, the most recent marker wins for that window.

## Tmux metadata

After a successful tmux launch, `orc` stamps the target window with user options
that identify the work without replacing the durable state contract:

- `@orc_ticket`
- `@orc_stage`
- `@orc_worker`
- `@orc_engine`
- `@orc_provider_engine`
- `@orc_provider_session`
- `@orc_feature_dir`

The exact agent pane is marked `@orc_agent=1` and receives the same identity
options. These options support live reverse lookup and safe targeting in a
multi-pane window. `STATE.yaml` remains authoritative if a marker is missing or
stale. Provider options are a live correlation overlay and are cleared when a
fresh launch does not have a resumable provider identity.

## Interactions

`orc watch` should be interactive, implemented as its own small Bubble Tea
program.

Keybindings:

| Key | Action |
|-----|--------|
| `j` / `down` | select next ticket |
| `k` / `up` | select previous ticket |
| `enter` | preview selected prompt |
| `a` | attach/focus selected agent session/window |
| `i` | attach to the next live session that needs attention |
| `n` | toggle the selected prompt preview |
| `r` | refresh immediately |
| `q` | quit watch pane |

Prompt sending must be explicit. Selecting a ticket or pressing `enter` should
not paste into an agent pane by itself.

## Attach/focus

Pressing `a` should focus the selected ticket's tmux target:

- Target session comes from `STATE.yaml runtime.tmux.session`.
- Target window comes from the current `STATE.yaml stage.name`.
- Inside tmux, use `tmux switch-client -t <session>:<window>`.
- Outside tmux, use `tmux attach-session -t <session>:<window>`.
- If no tmux runtime is recorded, or the session is known stopped, show a short
  in-watch status message instead of silently doing nothing.

Pressing `i` cycles to the next live `blocked`, `input`, or `review` row and
attaches to its exact session/window target. `orc focus` performs the same
attention-first action non-interactively, choosing the highest-priority target.

## Prompt actions

There are two prompt-related behaviors:

1. Preview or print the prompt for the selected ticket.
2. Send the prompt to the agent's tmux window/pane.

The rail should show a short selected-session detail block without requiring a
key press. Preview/details can still expand the selected story when the user
wants more context; this expanded page should include recent history. Send-to-agent
can follow after the target window logic is reliable.

When a ticket is blocked because `STATE.yaml` is `paused`, label the prompt text
as `Blocker`. For non-blocked states, label it as `Next`.

When sending, `orc` should route through shared Go tmux helpers rather than
constructing shell snippets in the TUI layer. This avoids quoting problems and
keeps behavior testable.

## Future structured human responses

For v1, blocked tickets should rely on freeform human-readable blocker text in
`STATE.yaml next_action.prompt`. `orc watch` should display that text, but it
should not guess the correct reply for yes/no, approval, or choice prompts.

A future `STATE.yaml` contract could add optional structured response hints under
`next_action`:

```yaml
next_action:
  worker: human
  prompt: "Approve opening the PR?"
  response:
    type: choice
    options:
      - value: "yes"
        label: "Approve"
      - value: "no"
        label: "Do not approve"
```

Other possible response types:

```yaml
response:
  type: text
  placeholder: "Enter the target refresh token TTL"
```

```yaml
response:
  type: confirm
  yes: "yes"
  no: "no"
```

With structured response hints, `orc watch` could render a compact reply screen
and send the exact configured value to the agent. Without those hints, reply/send
should remain manual and explicit.

## tmux toggle

Users should be able to open and close the dashboard with one tmux binding.

Example binding:

```tmux
bind-key O run-shell "orc watch --tmux-toggle"
```

Toggle behavior:

1. If an `orc watch` pane exists in the current tmux window, close it.
2. Otherwise split a pane using `--tmux-layout` and `--tmux-size`.
3. Run `orc watch`.
4. Mark the pane so future toggles can find it.

Implementation approach:

- Set a pane marker such as `@orc_watch=1`, or use a stable pane title.
- Find existing panes with `tmux list-panes`.
- Open with a command similar to:

```sh
tmux split-window -h -l 32 "orc watch"      # default right-side pane
tmux split-window -v -l 25% "orc watch"     # bottom pane
```

The exact command should live in Go so it can respect workspace root, current
ticket context, and future flags. When `--workspace` is not explicitly provided,
Orc should find the workspace by walking upward to `orc.yaml`; if it cannot find
one, it should return a clear error. The spawned watch pane should receive the
resolved workspace path explicitly, for example `orc --workspace <root> watch`,
so tmux pane CWD does not matter.

## Future tmux control modes

`orc watch` can eventually support three tmux control modes:

1. Side pane mode: `orc watch --tmux-toggle` runs inside tmux and opens/closes
   the watch rail next to the active work.
2. Standalone attach mode: `orc watch` runs in its own terminal, and an attach
   action takes over that terminal with `tmux attach -t <session>:<window>`.
3. Remote controller mode: `orc watch` runs in its own terminal and asks an
   already-attached tmux client in another terminal to switch focus with
   `tmux switch-client -c <client> -t <session>:<window>`.

For remote controller mode, `<client>` is a tmux client identifier. In practice
this is usually the client TTY path reported by:

```sh
tmux list-clients -F "#{client_tty}\t#{session_name}"
```

Example values look like `/dev/ttys003`, `/dev/pts/4`, or whatever terminal
device tmux reports for the attached client. If exactly one tmux client is
attached, `orc watch` could infer it. If multiple clients are attached, the user
should choose one or pass a future flag such as `--tmux-client /dev/ttys003`.

## Implementation outline

Add a new command and package:

```text
cmd/orc/watch_cmd.go
internal/watch/
  model.go
  data.go
  render.go
```

Reuse existing packages where possible:

- `internal/featurelist` for session rows.
- `internal/ticket` and `internal/ticketview` for focused details.
- `internal/tmux` for session/window lookup, optional attention markers,
  attach, split, close, and send.
- Existing next/runner logic for prompt generation.

Add tmux helpers as needed:

- Check session/window liveness.
- Read window attention marker when `tmux-attention` state is available.
- Split a watch pane.
- Mark/find/close a watch pane.
- Send prompt text safely to an agent target.

## Phased plan

### Phase 1: passive watch rail

- Add `orc watch`.
- Render compact multi-session and single-ticket views.
- Show a small selected-session detail section below the compact list.
- Refresh on an interval.
- Show durable `STATE.yaml` state as the primary status.
- Show tmux liveness as `live` or `stopped` for configured sessions.
- Optionally read `@agent_attention` as a backup/live overlay for active work.
- Quit with `q` or Ctrl-C.

### Phase 2: tmux toggle

- Add `orc watch --tmux-toggle`.
- Add tmux pane marker/finder.
- Document a tmux binding.
- Ensure repeated toggles do not create duplicate watch panes.

### Phase 3: prompt preview and attach

- Add selection.
- Add expanded story details with prompt and recent history.
- Add attach/focus action.
- Keep send-to-agent out until target detection is solid.

### Phase 4: send-to-agent

- Add explicit prompt send action.
- Confirm target session/window/pane before sending when ambiguous.
- Add tests for shell quoting and tmux target resolution.

### Phase 5: notification emission

- Optionally emit `tmux-attention` markers from `orc mark`.
- Reuse the broader notification plan for configurable event hooks.

### Future: remote tmux controller

- Add attached-client discovery with `tmux list-clients`.
- Add optional `--tmux-client <client>` for standalone watch windows.
- Let attach/focus switch another tmux client when explicitly configured or when
  exactly one client is attached.

### Future: structured human replies

- Extend `STATE.yaml next_action` with optional response metadata.
- Render choice/confirm/text reply screens in `orc watch`.
- Send the configured response value to the agent only after explicit user
  confirmation.

## Open questions

- Should `orc watch` default to all active tickets, or infer the current ticket
  from the tmux session/window when possible?
- Should `q` only quit the watch process, or close the watch pane when running
  inside a pane opened by `--tmux-toggle`?
- Should prompt preview replace the rail temporarily, or open in a wider tmux
  popup/pane?
- Should `--wide` be automatic based on terminal width?
- Should `orc watch` show `@agent_attention` as the primary label for active
  tickets, or as a secondary hint next to the durable status?
- Should standalone `orc watch` infer a single attached tmux client for remote
  focus, or require `--tmux-client` to avoid surprising focus changes?
