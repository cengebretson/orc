# orc watch spec

`orc watch` is the Live entry point to Orc's shared dashboard. It is meant to
run as a compact tmux-side rail in a narrow split and gains Live/Workspace
section navigation when rendered wide.

## Goals

- Show active sessions and their current state at a glance in a 24-32 column pane.
- Let a user quickly select a ticket and act on its prompt.
- Keep `STATE.yaml` as the durable source of truth.
- Optionally consume `tmux-attention` as a transient backup signal.
- Keep Workspace as the broad configuration, health, and detail browser within
  the shared dashboard application.

## Non-goals

- Do not build a full terminal multiplexer.
- Do not require `tmux-attention`.
- Do not infer complex semantic agent state from terminal output in v1.
- Do not make prompt sending automatic on selection.
- Do not force the Workspace layout into the narrow rail.

## Command shape

```sh
orc watch
orc watch PROJ-123
orc watch --interval 2s
orc watch --wide
orc watch --demo
orc watch --demo --wide
orc watch --view pet
orc watch --view pet --pet-layout column
orc watch --tmux-toggle
orc watch --view pet --tmux-toggle
orc watch --tmux-toggle --tmux-layout bottom --tmux-size 25%
orc dashboard
orc focus
```

`orc watch` defaults to a compact multi-ticket rail. `orc watch PROJ-123`
focuses a single ticket and can show a little more context. `--wide` renders the
richer selected-row highlight and selected-work card around a table-like
session view, with worker shown as a column in the session list
rather than as a separate list. Pressing `enter` opens the selected story
details page in both compact and wide layouts, including recent story history.
Pressing `v` toggles the same rows and controls into an
animated little-orc pet view; `--view pet` starts there directly. In pet mode,
`l` toggles the responsive grid and a vertical column; the matching startup
flag is `--pet-layout column`. `--tmux-toggle` is a helper for tmux keybindings.
By default it opens a 32-column right-side pane; `--tmux-layout bottom` and
`--tmux-size <size>` can override the split.

`--demo` replaces live workspace rows with four read-only synthetic tickets so
the complete visual language can be reviewed without manufacturing provider or
tmux state. The demo covers active, blocked, stopped, and completed work;
green/yellow/red context pressure and trends; workflow progress; attention;
history; and the completion treatment. Attach and focus are disabled in demo
mode. Demo mode does not require an initialized workspace.

At 56 columns and wider, watch runs inside the shared dashboard shell. `[` opens
Live and `]` opens Workspace; switching preserves each section's selection,
filter, drill-in, and scroll state. Below that width, adaptive watch renders the
Live rail directly with no extra shell chrome. `orc dashboard` uses the same
application but starts in Workspace. The dashboard navigation replaces the
former `ORC WATCH` banner in wide layouts.

See [Tmux integration](tmux.md) for copyable watch popup and split bindings,
workspace selection, and syntax verification. Orc never installs those bindings.

Press `/` to filter the rail without changing durable state or session
priority. The shared matcher covers ticket, slug, workflow, stage, worker,
engine, repository, branch, status, and attention state. Enter keeps the active
filter; Escape clears it.

## Little-orc pet view

Pet mode is an additive presentation over the same selected row, filtering,
preview, attach, focus, and refresh behavior as the rail. It never mutates
durable state or launches an agent. The rail remains the default.

Each ticket gets a deterministic green or teal little orc. Pointed ears, tusks,
skin tone, and animation phase make the creature recognizable across refreshes.
The compact grid automatically fits one to four pets per row as the terminal
widens; a 100-column view fits three without dropping status or context details.
Column layout forces one card per row, including in wide panes; it is
presentation-only and can be toggled without restarting watch.
Only the selected pet draws a rounded border; unselected cards retain the same
footprint so the grid does not jump as focus moves.
The visual state comes from Orc's existing durable and live signals:

| Orc signal | Little-orc behavior |
|---|---|
| `pending` or `ready` | Wobbling egg |
| Active live work | Hammering, bouncing orc |
| Live provider reports idle/waiting | Sleeping orc |
| Input or review attention | Alert, pulsing orc |
| Paused or blocked | Stuck, unhappy orc |
| Active ticket with stopped tmux | Faded offline orc |
| Done | Celebrating orc |
| Invalid state | Orc needing care |

Yellow and red context pressure add tired and exhausted labels without replacing
the underlying state. Narrow panes show the selected creature; wider terminals
use a responsive card grid with repository/worktree room labels. `j`/`k`, `/`,
`enter`, `a`, `i`, `r`, `Esc`, and `q` retain their rail meanings.

Animation uses an independent low-cost terminal tick only while pet mode is
visible. Toggling back with `v` stops that tick; the default rail keeps its
existing data-refresh cadence.

## Narrow rail view

The default view should assume a narrow vertical split. Avoid tables, headers,
long labels, and dense detail.

Example multi-session rail:

```text
╭─ orc  workspace ─────────╮
│ ● 2 RUNNING  ◐ 1 PAUSED │
│ ! 1 NEEDS YOU           │
│ ↺ 3s ago                 │
╰──────────────────────────╯

▌ ! PROJ-123
  ● PROJ-124
  × PROJ-125

NEEDS YOU
╭──────────────────────────╮
│  ! BLOCKED               │
│   api/feature · develop  │
│   bob-dev                │
│   tmux live              │
│   ctx ████░░ 78%         │
│   updated 4m ago         │
│                          │
│ Blocker                  │
│ fix tests                │
╰──────────────────────────╯

```

Example focused ticket view:

```text
╭─ orc  workspace ─────────╮
│ ● 1 RUNNING  ◐ 1 PAUSED │
│ ! 1 NEEDS YOU           │
│ ↺ 3s ago                 │
╰──────────────────────────╯

PROJ-123

▌ ! PROJ-123

NEEDS YOU
╭──────────────────╮
│  ! BLOCKED       │
│   develop        │
│   bob-dev        │
│   tmux live      │
│                  │
│ Blocker          │
│ fix tests        │
╰──────────────────╯
```

The renderer should truncate cleanly to the available width. The primary signal
is the state icon/label; the selected session uses a full-width highlighted row
and a bordered details card so it remains obvious even without terminal color.
The compact overview card mirrors the Workspace dashboard header, separates
live sessions from work needing attention, and shows the age of the latest
refresh. It replaces the former full-width title banner.
Within the details block, keep status on the first line, combine non-workspace
repository room and stage when both are available, indent worker/tmux/context
metadata, and render `Blocker` or `Next` as its own section heading before the
prompt text. Activity age prefers matched provider telemetry and falls back to
the latest durable story history entry; it is not inferred from terminal output.
Persistent control legends are intentionally omitted from watch layouts; `?`
opens the complete navigation reference without consuming dashboard space.
Inside expanded details, `j`/`k` and the arrow keys scroll by line,
Page Up/Down and `Ctrl-U`/`Ctrl-D` scroll by half-page, and `g`/`G` jump to the
top or bottom. This keeps long workflow histories fully reachable.

The selected card and expanded details page render the configured workflow as a
progress route, for example `✓ intake → ● develop → ○ review → ○ ship`. Stage
order and aliases come from `orc.yaml`; current-stage labels use those aliases
too, while loop stages are shown at their configured owner when active. The
expanded page uses the feature name and ticket as the
first panel title, consolidates provider and live runtime metadata into that
panel, then embeds Workflow, Next Action, and History titles into their
respective panel borders without a separate page header.
It includes the current workflow position, next stage and automatic/manual
advance mode, time in the current stage and visit count, repository room/branch,
provider/model, exact tmux target, attention, context trend, and last activity
when available. History uses a connected event timeline and shows up to the 12
most recent entries.

The rail uses a lightweight presentation tick for a live-work pulse, two-second
state-change highlight, and four-second completion treatment. This does not
reload workspace or provider data. Durable/live data still refreshes every five
seconds by default, `--interval` changes that cadence, and `r` refreshes
immediately. Context sparklines retain the ten most recent refresh samples in
memory and do not modify durable state.

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
context usage as a percentage. The Workspace section uses the same classifier in its
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

`orc watch` is the Live section of the shared Bubble Tea dashboard. Narrow
terminals keep the compact rail without dashboard chrome; wider terminals can
switch to Workspace with `[` and `]` without losing Live selection or details.

Keybindings:

| Key | Action |
|-----|--------|
| `j` / `down` | select next ticket |
| `k` / `up` | select previous ticket |
| `/` | filter visible work |
| `enter` | preview selected prompt |
| `a` | attach/focus selected agent session/window |
| `i` | attach to the next live session that needs attention |
| `n` | toggle the selected prompt preview |
| `v` | toggle the rail and little-orc pet view |
| `l` | toggle responsive and column pet layouts (pet view only) |
| `r` | refresh immediately |
| `j` / `k`, arrows | scroll while details are open |
| `pgup` / `pgdown`, `ctrl+u` / `ctrl+d` | scroll details by page or half-page |
| `g` / `G` | jump to the top or bottom of details |
| `esc` | close details or clear filtering; quit when already at the top level |
| `[` / `]` | switch between Live and Workspace in the wide dashboard |
| `q` | quit watch pane |
| `?` | toggle the compact key help overlay |

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
constructing shell snippets in the dashboard layer. This avoids quoting problems and
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

## Delivery status

### Phase 1: passive watch rail — completed

The compact multi-session and single-ticket views refresh on an interval, keep
durable `STATE.yaml` state primary, layer tmux liveness and attention on top, and
provide selected-session details plus clean `q`/Ctrl-C exit behavior.

### Phase 2: tmux toggle — completed

`orc watch --tmux-toggle` uses exact pane markers to open or close one watch pane
with configurable right/bottom layout and size. Repeated toggles do not create
duplicate watch panes, and the user-owned binding is documented in
[`docs/tmux.md`](tmux.md).

### Phase 3: prompt preview and attach — completed

Selection, expandable prompt/history details, exact-pane attach, next-attention
focus, shared filtering, and immediate refresh are implemented and covered by
rendering and tmux-target tests.

### Phase 4: optional little-orc pet view — completed

`v` toggles between the default rail and an animated Tamagotchi-style view;
`--view pet` selects it at startup and is forwarded through `--tmux-toggle`.
`l` / `--pet-layout column` force a vertical card stack; startup choices are
also forwarded through `--tmux-toggle`.
Both modes share data, selection, filtering, preview, attach, focus, refresh,
and exit behavior. Animation runs only while the pet view is active.

Sprites were later redrawn as true-color half-block pixel art. An optional
three-row micro sprite size shipped alongside the original ASCII sprites but
was removed once pixel art replaced them — a detailed creature (ears, tusks,
mohawk) doesn't hold up at that resolution, so there is now a single sprite
size.

### Post-v1: direct prompt sending

Directly sending previewed prompt text to an agent pane is intentionally outside
the v1 boundary. `orc next` and `orc jit` remain the canonical launch surfaces;
watch selection, preview, or attach never sends text by itself. A future send
action must be explicit, confirmed, routed through the shared exact-pane helper,
and covered for shell quoting and ambiguous targets.

### Later: notification emission

Configurable completion and blocker hooks are tracked in [`plan.md`](../plan.md).
They may optionally mirror tmux-attention markers after durable state is written,
but notifications do not block v1.

### Post-v1: remote tmux controller

Remote attached-client discovery and `--tmux-client <client>` selection remain
deferred. Standalone watch/focus processes do not guess which other tmux client
to move.

### Post-v1: structured human replies

Choice, confirmation, and text response schemas may eventually extend
`STATE.yaml next_action`. Any value sent to an agent must require explicit human
confirmation.

## Resolved v1 behavior

- `orc watch` without a ticket shows all active work; an optional ticket scopes
  the rail explicitly.
- `q` exits the watch process. A pane created by `--tmux-toggle` closes naturally
  when that process exits.
- Prompt preview expands inside the rail and returns with `Esc` or `enter`.
- The compact/wide layout responds to terminal width; `--wide` forces the wider
  table.
- The optional little-orc view is selected with `v` or `--view pet`; the rail
  remains the default and its refresh path does not run animation ticks.
- Pet layout is presentation-only: `l` / `--pet-layout` control responsive
  versus column card placement.
- `STATE.yaml` remains authoritative. `@agent_attention` is a live urgency hint,
  never durable workflow state.
- Attach and focus target the current tmux context only. Remote-client movement
  remains explicit post-v1 work.
