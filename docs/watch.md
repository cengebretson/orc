# orc watch spec

`orc watch` is Orc's dedicated Live entry point. It is meant to run as a compact
tmux-side rail in a narrow split and uses its richer table layout when rendered
wide; workspace exploration lives in `orc dashboard`.

## Goals

- Show active sessions and their current state at a glance in a 24-32 column pane.
- Let a user quickly select a ticket and act on its prompt.
- Keep `STATE.yaml` as the durable source of truth.
- Optionally consume `tmux-attention` as a transient backup signal.
- Keep the dashboard's Features tab as the durable work browser while sharing
  Live session state, attention, tmux, and context telemetry.

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

At 56 columns and wider, watch uses its wide Live table and selected-work panel.
Below that width it renders the compact rail. It remains separate from dashboard
navigation at every width. `orc dashboard` starts in the merged Live tab and
folds the same operational session cues into its Features table and shared
banner.

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

The default view assumes a narrow vertical split. It avoids tables, headers,
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

The renderer truncates cleanly to the available width. The primary signal
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

`orc watch` is driven primarily by the durable Orc workflow contract.
`STATE.yaml` is authoritative for whether work needs a human, is done, is
pending, or is active. Live runtime checks only refine that durable view.

The important contract point is that an agent needing human input is not just a
terminal condition. The agent must run:

```sh
orc mark <ticket> pause "<what you need>"
```

before asking the human. That makes `STATE.yaml` show `paused`, and `orc watch`
displays that as blocked/human-needed without requiring any tmux marker.

`orc watch` combines these sources:

- `STATE.yaml`: ticket, slug, workflow, stage, worker, status, next action.
- tmux: session/window exists or stopped.
- `tmux-attention`: optional live marker from `@agent_attention`, read per pane
  and rolled up per window.
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
- tmux options: `@agent_attention`, and optionally `@agent_attention_since`
  (epoch seconds) for how long that state has been held.
- status rendering through `@tmux_attention_status`.
- Claude/Codex hooks that call the CLI.
- clear-on-view behavior through tmux hooks.

`orc watch` reads the marker from each pane in the window:

```sh
tmux list-panes -t <session>:<window> -F '#{@agent_attention}	#{@agent_attention_since}'
```

Reporters should prefer setting it on the pane they run in:

```sh
tmux set-option -p -t "$TMUX_PANE" @agent_attention blocked \; \
     set-option -p -t "$TMUX_PANE" @agent_attention_since "$(date +%s)"
```

Setting it on the window still works — tmux resolves `@` options through
pane → window → session, so a pane with no value of its own reports the
window's.

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

When `STATE.yaml` says `paused`, watch shows `blocked` even if
`@agent_attention` is empty or stale. When `STATE.yaml` says `active`,
`@agent_attention=input|blocked|review|done` can refine the live display.

### Windows with more than one agent

A window can host more than one agent — an `orc jit --tmux` task sent into a
stage's existing session, or a split you made yourself — and each reports
independently. `orc` reads every pane in the window and rolls them up:

- **The most urgent state wins**, ordered `blocked` → `input` → `review` →
  `done`. Blocked work has stopped and cannot continue; `input` is stopped but
  answerable; `review` is finished work awaiting a decision; `done` needs
  nothing. A blocked agent is therefore never hidden behind a finished one.
- **Ties take the earliest `@agent_attention_since`**, so the elapsed time
  tracks whichever agent has been waiting longest rather than whichever most
  recently changed. A pane that reports a state without a time is treated as
  unknown, not as the epoch.
- **Unrecognized values are no signal.** Only the four states above are
  displayed; anything else reads as no marker at all.

A single-pane window behaves exactly as it always has.

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

`orc watch` is the dedicated Live Bubble Tea view. Narrow terminals use the
compact rail; wider terminals use the richer Live table without dashboard tabs.

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
| `q` | quit watch pane |
| `?` | toggle the compact key help overlay |

Prompt sending must be explicit. Selecting a ticket or pressing `enter` does
not paste into an agent pane by itself.

## Attach/focus

Pressing `a` focuses the selected ticket's exact recorded runtime target:

- Target identity comes from `STATE.yaml runtime.mux`, with legacy tmux state
  retained only as a compatibility fallback.
- The selected backend validates the exact workspace, tab, and pane before
  attaching; focus is never inferred from a label or active pane.
- If no runtime is recorded, the backend is unavailable, or the target is
  stopped, the rail shows a short status message instead of guessing.

Pressing `i` cycles to the next live `blocked`, `input`, or `review` row and
attaches to its exact runtime target. `orc focus` performs the same
attention-first action non-interactively, choosing the highest-priority target.

## Prompt actions

The rail shows a short selected-session detail block without requiring a
key press. Preview/details can still expand the selected story when the user
wants more context; this expanded page includes recent history.

When a ticket is blocked because `STATE.yaml` is `paused`, label the prompt text
as `Blocker`. For non-blocked states, label it as `Next`.

Watch does not send text. `orc next`, `orc jit`, and structured `orc ctl`
commands are the canonical launch/control surfaces. Prompt sending from Watch
is roadmap work and must be explicit, confirmed, exact-targeted, and covered
for quoting and ambiguous targets.

## tmux toggle

Users can open and close the dashboard with one tmux binding.

Example binding:

```tmux
bind-key O run-shell "orc watch --tmux-toggle"
```

Toggle behavior:

1. If an `orc watch` pane exists in the current tmux window, close it.
2. Otherwise split a pane using `--tmux-layout` and `--tmux-size`.
3. Run `orc watch`.
4. Mark the pane so future toggles can find it.

Orc marks the owned pane, finds it with tmux pane inventory, and opens a split
equivalent to:

```sh
tmux split-window -h -l 32 "orc watch"      # default right-side pane
tmux split-window -v -l 25% "orc watch"     # bottom pane
```

When `--workspace` is not explicit, Orc walks upward to `orc.yaml` and passes
the resolved workspace to the spawned pane so its initial CWD does not matter.

## Behavior

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
- Attach and focus target the selected ticket's recorded runtime. They never
  guess which unrelated remote tmux client to move.
