# Workflow Configuration

`orc.yaml` is the workspace configuration file. It defines the repos a workspace
can use, the named workflows tickets can follow, and the default safety settings
for launching and archiving work.

Workflow policy belongs in `orc.yaml` and worker markdown files. `orc` enforces
generic state transitions and safety rules around that policy.

## File Shape

```yaml
settings:
  default_workflow: default:standard
  artifact_policy: warn
  auto_archive: false
  auto_tmux: false
  auto_next: false
  workspace_refresh: 60
  theme: catppuccin-mocha
  context_pressure:
    green: 0
    yellow: 70
    red: 90
  notify:
    on: [blocked, complete]
    command: "notify-send 'orc' '{{ticket}} {{event}}'"
  parking:
    auto_park: [paused]
    wake_on: [status_change, attention, stage_change]
  rail:
    stuck_after: 15m
  herdr:
    task_cell:
      test_command: "make test"
      watch: true

repos:
  - name: my-app
    path: /Users/example/workspace/my-app
    purpose: Application code, APIs, tests
    worktree_setup: "{{repo_path}}/scripts/setup-worktree.sh -b {{branch}} --path {{worktree_path}}"
    agent_hints:
      - Use the repo Makefile before direct tool commands.

routing:
  - labels: [application]
    components: [web]
    repos: [my-app]
  - labels: [full-stack]
    repos: [my-app, shared-api]

aliases:
  workflows:
    adhoc: default:adhoc
    default: default:standard
  stages:
    adhoc: default:adhoc
    develop: default:develop
  workers:
    bob: default:bob

workflows:
  default:adhoc:
    description: Standalone local work created by orc run
    stages:
      - name: default:adhoc
        worker: default:bob
        advance: auto

  default:standard:
    description: General feature workflow — intake → develop → PR → QA
    stages:
      - name: default:intake
        worker: default:fred
        advance: auto
      - name: default:develop
        worker: default:bob
        advance: manual
        required_artifacts:
          - PLAN.md
          - develop/HANDOFF.md
        loop:
          via: default:code-review
          worker: default:zach
          max: 3
          on_max: pause
      - name: default:pr-open
        worker: default:bob
        advance: manual
      - name: default:qa-automation
        worker: default:brian
        advance: auto
```

Repository routes use exact, case-insensitive ticket labels or components and
may select one repository or an explicit cross-repository set. Signals must be
unique across routes. Multiple matching rules are ambiguous rather than merged;
put an intentional cross-repo selection in one rule. Ticket ID prefixes are not
repository selectors, and routes do not restrict which workflows may operate on
a repository.

## Settings

| Field | Required | Meaning |
|-------|----------|---------|
| `default_workflow` | Yes, when workflows exist | Workflow used by `orc work <ticket>` when `--workflow` is omitted. |
| `artifact_policy` | No | `warn` (default) reports missing/empty core docs and stage artifacts without blocking. `block` makes `orc mark <ticket> next` refuse to advance until they are ready. |
| `auto_archive` | No | Archives tickets automatically after the last stage completes. |
| `auto_tmux` | No | Uses tmux for new ticket launches by default. Same intent as `orc work --tmux`. |
| `auto_next` | No | Launches the first stage immediately after `orc work`. Same intent as `orc work --next`. |
| `workspace_refresh` | No | Workspace dashboard refresh interval in seconds. Defaults to 60 when unset or zero. |
| `theme` | No | Dashboard color theme. Defaults to `catppuccin-mocha`. |
| `quotes` | No | Optional dashboard status quotes. |
| `context_pressure` | No | Green, yellow, and red percentage boundaries for live provider context usage in `orc watch` and `orc dashboard`. Defaults to `0`, `70`, and `90`; values must satisfy `0 <= green < yellow < red <= 100`. |
| `notify` | No | Best-effort command run after selected `blocked` or `complete` transitions. See below. |
| `parking` | No | Reversible display-only parking for Live views. `auto_park` selects statuses to hide; `wake_on` selects events that restore them. |
| `rail` | No | Presentation settings for the compact live rail. `stuck_after` is a positive duration after which working sessions render as stuck. |
| `herdr` | No | Native Herdr layout settings. `task_cell` can add test and watch panes beside a launched agent. |

### Transition notifications

`settings.notify.on` accepts `blocked`, `complete`, `error`, or `all`. Orc fires
`blocked` after a successful `orc mark <ticket> pause` and `complete` after a
successful stage advance or explicit `done`. `error` is reserved for a future
explicit failure transition.

The command runs through `/bin/sh` from the workspace root with a five-second
timeout. The placeholders `{{ticket}}`, `{{slug}}`, `{{event}}`, `{{stage}}`,
and `{{workflow}}` expand from the updated `STATE.yaml`. The same values are
available as `ORC_TICKET`, `ORC_SLUG`, `ORC_EVENT`, `ORC_STAGE`, and
`ORC_WORKFLOW`. Prefer the environment variables when values need shell-safe
handling.

Notification failures print a warning but do not roll back or fail the state
transition. An empty command or an event absent from `on` is a no-op.

### Automatic parking

`settings.parking.auto_park` accepts `pending`, `active`, `paused`, `done`, and
`archived`. When it is non-empty, `wake_on` must contain at least one of
`status_change`, `attention`, or `stage_change`. Parking affects Live views only;
it does not stop sessions, remove worktrees, or change workflow state. See
[Automatic reversible parking](sessions.md#automatic-reversible-parking) for
the operating behavior.

### Live rail

`settings.rail.stuck_after` controls when a continuously working session is
presented as stuck in `orc watch`, `orc rail`, and the dashboard. It affects
presentation only and never changes durable workflow state.

### Herdr task cells

`settings.herdr.task_cell` applies only to `--mux herdr` launches. A non-empty
`test_command` creates a test pane and runs the command from the resolved
worktree. `watch: true` creates an `orc watch` pane for the ticket. With both
enabled, tests and watch share a utility column beside the agent.

Orc records ownership metadata for utility panes and reuses exact owned panes;
it does not identify them from display labels. Task-cell setup failures warn but
do not prevent the agent from launching.

## Repos

`repos` describes source repositories available to the workspace.

| Field | Required | Meaning |
|-------|----------|---------|
| `name` | Yes | Stable repo identifier used in state and displays. |
| `path` | Yes | Path to the repo, usually relative to the workspace root. |
| `purpose` | No | Human-readable description of what belongs in the repo. |
| `worktree_setup` | No | Repo-specific setup command for ticket worktrees. `orc` prints the resolved command in launch prompts when the target worktree is missing; the agent runs it. |
| `agent_hints` | No | Short repo-specific guidance shown in `orc next` prompts for features using that repo. |

`worktree_setup` supports these placeholders: `{{branch}}`, `{{worktree_path}}`,
`{{repo_path}}`, `{{repo_name}}`, `{{ticket}}`, `{{slug}}`, and
`{{workspace}}`. Include `{{worktree_path}}` so the command creates the checkout
where `STATE.yaml` expects it. If omitted, `orc doctor` warns because the command
may create an untracked worktree.

## Routing

`routing` maps exact, case-insensitive ticket labels or components to configured
repository names. Ticket prefixes are not repository selectors. Multiple
matching rules are ambiguous and pause rather than merge; express intentional
cross-repository work as one rule naming every repository.

| Field | Required | Meaning |
|-------|----------|---------|
| `labels` | No | Ticket labels that select the route. |
| `components` | No | Ticket components that select the route. |
| `repos` | Yes | One or more repository names from `repos`. |

Intake persists the resolved selection in `STATE.yaml.repos`. When no route
matches, repository `purpose` descriptions provide the fallback context for
agent-driven selection.

## Aliases

`aliases` maps short display names to canonical pack-qualified IDs. It has
separate `workflows`, `stages`, and `workers` maps. Workflow and stage aliases
are also accepted by configuration and stage lookup paths that resolve those
IDs; worker aliases are used for display.

Within each map, only one alias may point to a given canonical ID. Keeping the
canonical IDs on the right preserves pack ownership while allowing concise UI
labels such as `default`, `develop`, and `bob`.

## Workflows

`workflows` is a map of workflow names to ordered stage lists. A ticket stores
its selected workflow in `STATE.yaml`. If the ticket omits `workflow`, `orc`
uses `settings.default_workflow`.

The bundled `default:adhoc` workflow is reserved for `orc run`. It has one
`default:adhoc` stage and completes the generated `LOCAL-N` feature when the
agent runs the exact
`orc mark LOCAL-N done --result "<summary of what was done>"` command included in
its prompt. `orc run` prompts for a worker when `--worker <id>` is omitted; the
selected worker is stamped into the feature without changing the configured
workflow.
`--repo <name>` records one configured repository in `STATE.yaml`; without the
flag, Orc selects the only configured repository or infers the checkout that
contains the current directory. If neither is possible, interactive use prompts
for a configured repository or the workspace root; non-interactive use fails
with the available flag values. `--attach` implies multiplexer launch and enters
the new session after the agent starts. Marking the feature done preserves its
session for inspection; `orc archive LOCAL-N` performs session cleanup.

Each workflow supports:

| Field | Required | Meaning |
|-------|----------|---------|
| `description` | No | One-line summary of what the workflow is for. Shown in the dashboard workflow detail. Packs set this so a pack's description survives install. |
| `stages` | Yes | Ordered list of stages (below). |

Each workflow stage supports:

| Field | Required | Meaning |
|-------|----------|---------|
| `name` | Yes | Stage identifier. Canonical pack stages use `<pack>:<stage>` and map to `stages/<pack>/<stage>.md`. |
| `worker` | Yes | Worker ID from `workers/<namespace>/*.md` that owns the stage by default. |
| `advance` | Yes | Completion mode. Valid values are `auto` and `manual`. |
| `required_artifacts` | No | Feature-folder files this stage should leave current before completion, such as `PLAN.md` or `develop/HANDOFF.md`. |
| `loop` | No | Optional repair/review loop attached to this stage. |

## Advance Modes

`advance: auto` means the agent should run `orc mark <ticket> next --result
"<summary>"` when the stage is complete.

`advance: manual` means the agent should run `orc mark <ticket> pause
"<summary>"` so a human can review before advancing.

## Loops

A `loop` attaches a non-linear repair or review stage to a main workflow stage.
The loop stage is not part of the normal stage order. It is entered only when the
owning stage sends the ticket there.

```yaml
- name: default:develop
  worker: default:bob
  advance: manual
  loop:
    via: default:code-review
    worker: default:zach
    max: 3
    on_max: pause
```

Loop fields:

| Field | Required | Meaning |
|-------|----------|---------|
| `via` | Yes | Loop stage name. |
| `worker` | Yes | Worker ID assigned to the loop stage. |
| `max` | No | Maximum loop count before `on_max` behavior applies. |
| `on_max` | No | Behavior when the loop count reaches `max`. `pause` (default) pauses for human review. `fail` marks the ticket done immediately. |
| `required_artifacts` | No | Feature-folder files expected from the loop stage before it returns to the owning stage. |

## Validation Expectations

Workspace configuration should satisfy these rules:

- `settings.default_workflow` names an existing workflow when workflows are configured.
- Every alias target is used by at most one alias within its workflow, stage, or worker map.
- `settings.parking.auto_park` contains valid statuses and has at least one valid `wake_on` condition when enabled.
- Every stage has a non-empty `name`.
- Stage names are unique within a workflow, including loop stage names.
- Every stage `worker` and loop `worker` names an existing file in `workers/`.
- `advance` is either `auto` or `manual`.
- `required_artifacts` entries are relative feature-folder paths and do not contain `..`.
- `loop.via` names a loop stage owned by exactly one workflow stage.
- `loop.on_max`, when set, is `pause` or `fail`.

Pack-authored workflows have one additional expectation: a pack should be
self-contained. A workflow inside a pack should only reference stages and workers
declared by that same pack. Once packs are installed, users can still edit the
workspace `orc.yaml` to intentionally compose a custom workflow from resources
provided by multiple packs.

`orc` validates this configuration in the paths that would otherwise route work:

- `orc doctor` reports invalid config under the `config` check.
- `orc doctor <ticket>` validates workspace config along with that ticket's `STATE.yaml`.
- `orc next <ticket>` refuses to launch when the workspace config is invalid.
- `orc mark <ticket> next` refuses to advance when the workspace config is invalid.
- If `settings.artifact_policy` is `block`, `orc mark <ticket> next` also
  refuses to advance when core feature docs are missing, empty, or directories,
  or when current-stage `required_artifacts` are missing, empty, directories, or
  still unchanged from the feature template. `--force` overrides the block and
  records the skipped artifacts in history.

## State Transitions

Agents should use `orc mark`; they should not hand-edit `STATE.yaml`.

| Command | Allowed From | Result |
|---------|--------------|--------|
| `orc mark <ticket> start` | `pending`, `ready` | Marks the ticket `active` and records the session start. |
| `orc mark <ticket> resume` | `paused` | Marks the ticket `active` and records the continuation. Clears the human-directed next action set by `pause`. |
| `orc mark <ticket> next --result "<summary>"` | `active`, `ready`, `paused` | Advances to the next workflow stage, returns from a loop stage, or marks the ticket `done` after the final stage. |
| `orc mark <ticket> next --stage <name> --worker <id>` | `active`, `ready`, `paused` | Moves to a configured workflow or loop stage and assigns the named worker. |
| `orc mark <ticket> pause "<reason>"` | Any non-final feature state | Marks the ticket `paused` and records why a human or external condition is needed. |
| `orc mark <ticket> done --result "<summary>"` | `active`, `ready`, `paused` | Explicitly closes active work. |

Transition validation rejects:

- `start` from `paused`; use `resume` to continue a paused ticket.
- `resume` from any status other than `paused`.
- `next` while a ticket is still `pending`; start the session first.
- `done` while a ticket is still `pending`.
- `next --stage` values that do not name a configured workflow or loop stage.
- `next --worker` values that do not name a worker file in `workers/`.
- `next` when `settings.artifact_policy: block` and the current stage's
  required artifacts are missing, empty, directories, or unchanged from the
  feature template — unless `--force` is passed to override for human review.

Use `orc artifacts <ticket>` to check the current stage's core docs and
`required_artifacts` without advancing. Add `--all` to inspect every required
artifact in the workflow, or `--json` for automation.

## Where to Put Policy

- Put stage order, default workers, and loop shape in `orc.yaml`.
- Put agent behavior, model choice, permissions, and launch defaults in `workers/*.md`.
- Put per-stage instructions in `stages/<stage>.md`.
- Put current ticket state in `features/<slug>/STATE.yaml`.
- Put code-level safety checks in `orc` internals only when they are generic across workflows.
