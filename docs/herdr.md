# Herdr integration

Herdr is an optional native multiplexer backend. Tmux remains Orc's default;
select Herdr per command with `--mux herdr`:

```sh
orc next STORY-123 --mux herdr
orc status STORY-123 --mux herdr
orc sessions --mux herdr
orc attach STORY-123
orc dashboard --mux herdr
```

The first launch creates one Herdr workspace for the ticket and tabs for its
workflow stages, starts a recognized Claude or Codex agent, and submits the
stage prompt through Herdr's agent API. When the target repository is
unambiguous, Orc uses Herdr's native worktree surface: a missing checkout is
created with `herdr worktree create`, while an existing recorded checkout is
reopened with `herdr worktree open`. Orc stores the repository, branch,
worktree path, and exact IDs Herdr returns before launching the worker:

```yaml
runtime:
  mux:
    backend: herdr
    workspace: w-01K2ABC
    tab: t-01K2DEF
    pane: p-01K2GHI
```

Later ticket-specific commands select the recorded backend automatically.
Explicit `--mux` remains useful for workspace-wide inventory and dashboards.
If Herdr or its server is unavailable, live inventory is empty and launch can
use Orc's normal foreground fallback.

For a new ticket, native creation is automatic when `orc.yaml` configures one
repository. A multi-repo ticket must already select a recorded worktree through
`next_action.cwd`, so Orc never guesses which checkout should own the Herdr
workspace. A repository-specific `worktree_setup` command remains authoritative
when its checkout is missing because it may perform setup beyond a raw Git
worktree operation. Once that checkout exists, Herdr can reopen it natively.

## Lifecycle and exact attach

`orc sessions --mux herdr --json` preserves Herdr's structured agent lifecycle
in `lifecycle` and reports exact backend/workspace/tab/pane targets. Durable
workflow status in `STATE.yaml` remains authoritative; live lifecycle never
advances an Orc stage.

`orc attach` validates and targets the recorded pane. Inside Herdr it focuses
that agent or tab; outside Herdr it resolves the pane's terminal and attaches
to that terminal explicitly. Archive cleanup accepts only a recorded exact
Herdr workspace ID, never a matching display label.

## Sidebar metadata

Orc reports metadata with `source=orc` on the workspace and agent pane. Tokens
include `ticket`, `workflow`, `repository`, `branch`, `stage`, `next_action`,
`worker`, `engine`, `model`, `provider_session`, and `feature_dir` when those
values are available.

Herdr owns sidebar layout and navigation. To display Orc's tokens, add or adapt
user-owned sidebar rows in `~/.config/herdr/config.toml`; Orc does not rewrite
that file:

```toml
[ui.sidebar.agents]
rows = [
  ["state_icon", "$ticket", "$stage"],
  ["agent", "$worker"],
]

[ui.sidebar.spaces]
rows = [
  ["state_icon", "$ticket", "$workflow"],
  ["branch", "$next_action"],
]
```

Sidebar width remains a Herdr preference. For a wider rail, a practical range
is `sidebar_width = 64`, `sidebar_min_width = 56`, and
`sidebar_max_width = 72`.

## Task-cell layout

Herdr can arrange optional Orc-owned utility panes beside each stage agent:

```yaml
settings:
  herdr:
    task_cell:
      test_command: "make test"
      watch: true
```

The agent remains on the left. A test pane occupies a 35% column on the right;
when watch is also enabled, `orc watch <ticket>` opens below tests in that
column. Either pane can be enabled independently. Both commands start in the
agent's resolved worktree cwd, while the watch command receives the workspace
path explicitly.

Orc marks these panes with `source=orc`, `task_cell=tests` or
`task_cell=watch`, and an `orc_task_cell_owner` token tied to the exact feature
directory. Later launches reuse exact matching panes and never adopt a user
pane merely because it has a `tests` or `watch` label. Layout failures are
reported as warnings without preventing the stage agent from launching.

## Native transition notifications

When a ticket whose selected runtime is Herdr blocks or completes, Orc also
publishes a Herdr session notification. Blocked transitions use Herdr's
`request` sound and completed transitions use `done`. Delivery is best-effort:
a missing Herdr server warns without rolling back the durable state change.

This native notification is independent of `settings.notify`. A configured
notification command still runs, so it can continue delivering events to a
desktop service, chat integration, or other user-owned destination.

Herdr's notification delivery defaults to off. Choose the destination in the
user-owned Herdr config before expecting the alert to appear:

```toml
[ui.toast]
delivery = "herdr" # or "terminal" / "system"
```

## Structured agent control

`orc ctl` delegates lifecycle reads, prompting, waiting, timeout handling, and
stall detection to Herdr while continuing to use Orc's exact recorded pane
identity:

```bash
orc ctl agent state --ticket ORC-9
orc ctl agent prompt --ticket ORC-9 "Review this diff" --wait --timeout 120s
orc ctl agent wait --ticket ORC-9 --until blocked --timeout 120s
```

Results are JSON on stdout and failures are JSON on stderr. `agent state` is a
one-shot read of Herdr's recognized lifecycle and does not move focus. A prompt
submitted from a non-working state that produces no observed lifecycle change
preserves Herdr's `agent_prompt_stalled` error code. Waits observe lifecycle
state rather than pretending to track an individual agent turn; if the agent is
already working, the active turn settling may satisfy the wait.

Accepted `--until` values are `idle`, `working`, `blocked`, `done`, and
`unknown`. Repeat the flag to accept multiple states. Omitting it uses Herdr's
settled defaults: `idle`, `done`, or `blocked`.
