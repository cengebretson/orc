# Watch and rail

`orc watch` is Orc's dedicated Live entry point. It is meant to run as a compact
tmux-side rail in a narrow split and uses its richer table layout when rendered
wide; workspace exploration lives in `orc dashboard`.

It shows live sessions and work needing attention without replacing the durable
state in `STATE.yaml`. Selection and previews are read-only; prompting always
requires an explicit compose, review, and confirmation flow. The optional
`tmux-attention` integration refines active-session presentation but is never
required.

## Commands

```sh
orc watch
orc watch PROJ-123
orc watch --interval 2s
orc watch --wide
orc watch --demo
orc watch --demo --wide
orc rail toggle
orc rail toggle --layout bottom --size 25%
orc dashboard
orc focus
```

`orc watch` defaults to a compact multi-ticket rail. `orc watch PROJ-123`
focuses a single ticket and can show a little more context. `--wide` renders the
richer selected-row highlight and selected-work card around a table-like
session view, with worker shown as a column in the session list
rather than as a separate list. Pressing `enter` opens the selected story
details page in both compact and wide layouts, including recent story history.
To run the rail beside your work as a tmux pane, use `orc rail` rather than
`orc watch` — see [Tmux integration](tmux.md#managed-rail).

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

## Narrow rail view

The default view is designed for a narrow vertical split. It prioritizes state,
ticket identity, the selected blocker or next action, and enough runtime detail
to act safely.

Example:

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

The selected row and bordered details card remain identifiable without color.
Press `enter` for workflow progress, runtime metadata, next action, and recent
history. Details scroll with `j`/`k`, arrows, Page Up/Down, `Ctrl-U`/`Ctrl-D`,
and `g`/`G`; `?` opens the complete key reference.

Watch refreshes durable and live data every five seconds by default. Use
`--interval` to change the cadence or `r` to refresh immediately. Presentation
animation and context trends stay in memory and never alter workflow state.

## Implementation contract

Day-to-day users can skip this section and continue to [Interactions](#interactions).
These rules define how the UI derives trustworthy state and targets exact agents.

### State model

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
- `tmux-attention`: optional live marker from `@agent_pane_attention` (falling
  back to `@agent_attention`), read per pane and rolled up per window.
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

### Context pressure

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

### tmux-attention integration

`tmux-attention` can provide a low-level tmux notification primitive:

- CLI states: `input`, `blocked`, `review`, `done`, `clear`.
- status rendering through `@tmux_attention_status`.
- Claude/Codex hooks that call the CLI.
- clear-on-view behavior through tmux hooks.

`TestLiveAttentionPluginContract` in `internal/tmux` drives the real CLI and
asserts Orc reads back what it wrote, so a rename on either side fails a test
instead of silently breaking the integration. It skips when the plugin is not
installed, and `ORC_TMUX_ATTENTION_CLI` points it at a specific copy.

Orc consumes this plugin for display and never as an authority — see
[ADR 0005](adr/0005-orc-and-tmux-attention-are-layered.md) for why the two stay
layered, and [ADR 0004](adr/0004-authoritative-sources-are-named.md) for what a
marker may and may not drive.

**The plugin owns the option schema.** It is released and versioned, and
`tmux-fzf-jump` already consumes it, so orc publishes into the plugin's names
rather than defining its own:

| Option | Meaning |
|---|---|
| `@agent_pane_attention` | pane-scoped state — the authoritative marker |
| `@agent_pane_attention_updated_at` | epoch seconds the state was last written |
| `@agent_pane_attention_source` | what set it |
| `@agent_pane_context_active` | `1` while an agent turn is running |
| `@agent_pane_context_override` | persistent pane-local ticket or project label |
| `@agent_pane_context_slug` | descriptive slug shown beside the project in detailed status |

`@agent_pane_context_active` is read as the `context` observation source and
outranks title and screen inference: an agent hook reported it, rather than Orc
guessing from a picture. Only an explicit `1` counts — empty means the plugin is
not installed, and a cleared value is a finished turn, and neither is evidence
Orc should settle a pane on. It stays an observation regardless, because Orc
cannot verify who wrote it.

Orc also writes the older `@agent_attention{,_since,_source}` trio for its own
readers and for reporters written against earlier versions. The two timestamp
fields are **not** interchangeable — `_updated_at` is the plugin's, `_since` was
orc's — and writing only one is what previously let each side read the other's
state but not its age.

`orc watch` reads the marker from each pane in the window, preferring the
plugin's fields and falling back to the legacy pair:

```sh
tmux list-panes -t <session>:<window> \
  -F '#{@agent_pane_attention}	#{@agent_pane_attention_updated_at}'
```

Reporters should prefer the plugin's CLI, which writes the pane fields, derives
the window summary, and wires up clear-on-view:

```sh
tmux-attention blocked --target "$TMUX_PANE" --source my-tool
```

Setting options directly also works and needs no plugin installed — orc does
this so its lifecycle tracking never depends on an optional tool. Write both
schemas if you do:

```sh
tmux set-option -p -t "$TMUX_PANE" @agent_pane_attention blocked \; \
     set-option -p -t "$TMUX_PANE" @agent_pane_attention_updated_at "$(date +%s)" \; \
     set-option -p -t "$TMUX_PANE" @agent_attention blocked
```

Writing only `@agent_attention` renders the tab glyph — that format resolves
through pane → window → session — but leaves the marker invisible to the
plugin's CLI and absent from `tmux-fzf-jump`'s attention picker, which read the
pane-scoped `@agent_pane_attention`.

Setting it on the window still works — tmux resolves `@` options through
pane → window → session, so a pane with no value of its own reports the
window's.

For panes Orc launches, the exact `STATE.yaml` ticket and feature slug are also
published through `@agent_pane_context_override` and
`@agent_pane_context_slug`. tmux-attention therefore renders the durable ticket
identity without relying on an agent to call `project set`. These writes are
harmless when the optional plugin is absent.

When Orc launches inside a Git worktree, it also writes the same identity to
`tmux-attention-context` under that checkout's absolute Git directory. This is
tmux-attention 0.12.0's private worktree metadata contract: it keeps the checkout
clean and lets non-Orc panes opened in the same worktree derive the ticket and
slug. A recorded branch prevents stale metadata from surviving a checkout to
unrelated work. Failure to write this display-only file is reported as a launch
warning and never blocks the agent.

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

### Multi-agent tmux windows

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

### Tmux metadata

After a successful tmux launch, `orc` stamps the target window with user options
that identify the work without replacing the durable state contract:

- `@orc_ticket`
- `@orc_stage`
- `@orc_worker`
- `@orc_engine`
- `@orc_provider_engine`
- `@orc_provider_session`
- `@orc_feature_dir`
- `@orc_feature_slug`

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
| `s` | answer the selected ticket's pending question, or compose, review, and explicitly confirm a free-text prompt when it has none |
| `n` | toggle the selected prompt preview |
| `r` | refresh immediately |
| `j` / `k`, arrows | scroll while details are open |
| `pgup` / `pgdown`, `ctrl+u` / `ctrl+d` | scroll details by page or half-page |
| `g` / `G` | jump to the top or bottom of details |
| `esc` | close details or clear filtering; quit when already at the top level |
| `q` | quit watch pane |
| `?` | toggle the compact key help overlay |

Prompt sending is explicit. Press `s` to compose text, `enter` to review it,
then `y` to send it to the selected exact agent instance. Selection, preview,
and `enter` alone never paste into an agent pane.

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

Watch sends text only through the compose/review/confirm flow. It requires a
live backend with prompt capability plus an exact recorded workspace, tab,
pane, agent ID, and instance ID. Delivery then uses the same backend control
contract as `orc ctl` and waits briefly for an authoritative lifecycle
acknowledgement; an unavailable, stopped, stalled, replaced, or legacy target
is reported without guessing or falling back to the selected pane.

## Rail pane management

Users can open and close the dashboard with one tmux binding.

Example binding:

```tmux
bind-key O run-shell "orc rail toggle"
```

Toggle behavior:

1. If an `orc watch` pane exists in the current tmux window, close it.
2. Otherwise split a pane using `--layout` and `--size`.
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
- `q` exits the watch process. A pane created by `orc rail` closes naturally
  when that process exits.
- Prompt preview expands inside the rail and returns with `Esc` or `enter`.
- The compact/wide layout responds to terminal width; `--wide` forces the wider
  table.
- `STATE.yaml` remains authoritative. `@agent_attention` is a live urgency hint,
  never durable workflow state.
- Attach and focus target the selected ticket's recorded runtime. They never
  guess which unrelated remote tmux client to move.
