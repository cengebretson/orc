# orc reference

Deep reference for the workspace `orc` scaffolds and the files it manages. For
the getting-started guide, command list, and concepts, see the
[README](../README.md). For workflow configuration, see
[workflows.md](workflows.md).

## Workspace layout

```
my-workspace/
  AGENTS.md          shared agent entrypoint (Claude + Codex)
  CLAUDE.md          imports AGENTS.md (Claude entrypoint)
  TOOLS.md           approved tools, MCP servers, external systems
  RULES.md           approval, state update, and cost rules
  SETUP.md           one-time setup — run with your agent after init
  .gitignore         excludes projects/ and worktrees/

  projects/          preferred main repo checkouts; external paths also work

  features/
    _template/       copied for each new ticket
      STATE.yaml     durable state machine for the ticket
      TICKET.md      ticket summary and acceptance criteria
      SPEC.md        context, scope, and open questions
      PLAN.md        approach and steps
      DECISIONS.md   decisions and rationale
      # stage subfolders such as develop/, code-review/, and pr-open/
      # are created by agents when those stages write outputs
    _archive/        completed features moved here by `orc archive`

  workers/
    _template.md     worker definition template
    default/
      fred.md        intake and documentation worker
      bob.md         implementation worker
      zach.md        code review worker
      brian.md       QA automation worker
      ada.md         architecture worker

  stages/
    default/
      intake.md        load ticket context — runs first for every ticket
      develop.md       implementation
      code-review.md   review implementation before opening PR
      pr-open.md       preflight checks, open PR, handoff for review
      pr-repair.md     fix CI failures, review feedback, conflicts
      qa-automation.md implement and run automated tests
    # provided by installed packs; plain markdown — no frontmatter,
    # flow control lives in orc.yaml

  packs/
    default/
      pack.yaml        pack manifest
      .orc-pack.yaml   install provenance
      workflow.yaml    pack workflow definitions
      stages/          source stage docs copied into stages/default/
      workers/         source worker docs copied into workers/default/

  orc.yaml           workspace config — repo routing, workflows, loop stages, settings
  ORC.md             agent state contract — read at session start

  worktrees/         git worktrees for ticket branches (gitignored)
```

## Workspace files

The root files are the shared context every agent reads before starting work. Each has a distinct owner and purpose.

| File | Owner | Purpose |
|------|-------|---------|
| `AGENTS.md` | shared | Entry point for all agents — contract map, repository command boundaries, and cross-repo team conventions. |
| `CLAUDE.md` | orc | Imports `AGENTS.md`. Claude's entrypoint — do not edit. |
| `ORC.md` | orc | State contract — status values, `orc mark` commands, STATE.yaml rules. Do not add team conventions here. |
| `TOOLS.md` | user | Approved tools, MCP servers, CLI commands, external systems. Fill in during setup. |
| `RULES.md` | user | What requires human approval before agents act — PR gates, cost limits, destructive operations. |
| `SETUP.md` | orc | One-time setup guide. Run with your agent after `orc init` to configure repos, workers, and tool policy. |
| `orc.yaml` | user | Routing and workflow config — repos, ticket metadata routes, stages, workers, loops, and settings. |

`AGENTS.md` is the entry point — it fans out to everything else. `ORC.md` and `CLAUDE.md` are orc-managed and should not be edited directly. Everything else is yours to configure and extend.

## Feature folder

Every ticket is a self-contained context pack under `features/<slug>/`. Stages read what the previous one wrote and write their own outputs to a named subfolder — so any agent can pick up mid-flight without asking anyone.

```
features/STORY-123/
  STATE.yaml          orc-managed — status, stage, worker, history
  TICKET.md           intake writes   →  all stages read
  SPEC.md             intake writes   →  develop, code-review read
  PLAN.md             intake writes   →  develop reads
  DECISIONS.md        any stage writes → any stage reads

  develop/
    HANDOFF.md        develop writes  →  code-review, pr-open read
  code-review/
    REVIEW.md         code-review writes → develop, pr-open read
  pr-open/
    PR.md             pr-open writes  →  pr-repair, qa-automation, human read
  qa-automation/
    PLAN.md           qa-automation writes and reads across sessions
    RUNS.md
    RESULT.md
```

The stage subfolder names match the stage names in `orc.yaml` — provenance is always unambiguous. If you need to find what `develop` produced, look in `develop/`.

| File | Written by | Read by |
|------|-----------|---------|
| `STATE.yaml` | orc | orc, all agents |
| `TICKET.md` | intake | all stages |
| `SPEC.md` | intake | develop, code-review |
| `PLAN.md` | intake | develop |
| `DECISIONS.md` | any stage | any stage |
| `develop/HANDOFF.md` | develop | code-review, pr-open, qa-automation |
| `code-review/REVIEW.md` | code-review | develop, pr-open |
| `pr-open/PR.md` | pr-open | pr-repair, qa-automation, human |

## orc.yaml

`orc.yaml` is the workspace config. It declares repos, named workflows, loop
stages, and optional settings. See [workflows.md](workflows.md) for the full
configuration reference.

```yaml
settings:
  default_workflow: default:standard
  artifact_policy: warn
  auto_archive: false
  auto_tmux: false       # wrap every orc next launch in a tmux session automatically
  auto_next: false       # orc work immediately launches the first stage (same as --next)
  workspace_refresh: 60  # Workspace auto-refresh interval in seconds
  theme: catppuccin-mocha
  context_pressure:      # optional; defaults shown
    green: 0
    yellow: 70
    red: 90
  notify:                # optional transition notification command
    on: [blocked, complete]
    command: "notify-send 'orc' '{{ticket}} {{event}}'"
  herdr:                 # optional native Herdr layout
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

workflows:
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
        loop:
          via: default:pr-repair
          worker: default:bob
          max: 3
          on_max: pause
      - name: default:qa-automation
        worker: default:brian
        advance: auto
```

`settings.herdr.task_cell` applies only to `--mux herdr` launches. A non-empty
`test_command` creates a test pane and runs the configured shell command from
the agent's resolved worktree cwd. `watch: true` creates an `orc watch` pane for
the ticket. With both enabled, the agent occupies the left side while tests and
watch share a 35% utility column on the right. Orc stamps the utility panes with
ownership metadata tied to the exact feature directory and reuses them on later
launches; it never identifies them from display labels alone. Task-cell setup
failures are warnings and do not prevent the agent from launching.

`default_workflow` is used by `orc work <ticket>` when `--workflow` is omitted.
If it is not set, `orc work` returns an error. `advance: auto` tells agents to
run `orc mark <ticket> next` when a stage is complete; `advance: manual` tells agents to
run `orc mark <ticket> pause` so a human can review before continuing.

Routing rules map exact ticket labels or components to one or more configured
repository names. Ticket prefixes are intentionally not repository selectors:
one ticket namespace may span many repos. Intake records the resolved selection
in `STATE.yaml.repos`. Multiple matching rules are ambiguous and must pause;
intentional cross-repo work is expressed by one rule naming multiple repos.
Repository purpose is the fallback when no rule matches.

Repos may define `worktree_setup` when raw `git worktree add` is not enough.
`orc next` resolves supported placeholders and prints the command for the agent
when the target worktree is missing. `orc` does not execute the command itself.
Repos may also define `agent_hints`; `orc next` includes those hints when the
current feature references that repo.

Stages may define `required_artifacts` as feature-folder relative paths. `orc
next` reminds agents to keep those files current, and `orc validate` warns when
they are missing or empty. Loop stages can declare their own
`required_artifacts` under `loop`.
Set `settings.artifact_policy: block` to make `orc mark <ticket> next` refuse
to advance when core docs are missing or empty, or when current-stage
`required_artifacts` are missing, empty, or still byte-identical to the feature
template (reported as `unchanged from template` — nobody has written the doc
yet). Pass `orc mark <ticket> next --force` to override the block for human
review; the skipped artifacts are recorded in the stage result and history. The
default is `warn`.

`settings.context_pressure` controls the percentage boundaries used to color
live provider context usage in `orc watch` and `orc dashboard`. The values must obey
`0 <= green < yellow < red <= 100`. If the provider does not report a context
limit, Orc displays `n/a`; this live overlay never changes workflow state.

## Packs

`orc init` installs the built-in `default` pack unless you pass
`--skip-default-pack`. A pack is a reusable bundle of workflow definitions,
stage docs, worker docs, and suggested aliases.

There are three pack forms:

- Embedded pack: shipped inside the `orc` binary. Discover with `orc pack available`.
- Installed pack: a snapshot copied into `packs/<name>/` inside a workspace.
- Local path pack: a pack directory on disk, usually used with `orc pack inspect`
  before `orc pack install`.

`orc pack inspect <path>` validates a local pack without installing it. `orc
pack list` shows installed snapshots, the source they came from, and which
workflows are active. `orc pack show <pack>` shows one installed snapshot in
detail.

Packs are expected to be self-contained: every workflow a pack provides should
reference only stages and workers provided by that same pack. After install, the
workspace owns `orc.yaml`, so users may intentionally compose their own workflows
from stages and workers installed by multiple packs.

Use `orc pack install` to add packs after initialization:

```bash
orc pack install default
orc pack install ./packs/hotfix
```

Install copies a snapshot to `packs/<name>/`, writes provenance to
`packs/<name>/.orc-pack.yaml`, materializes runtime files into `stages/` and
`workers/`, and merges non-conflicting workflow and alias entries into
`orc.yaml`. The installed snapshot is provenance, while the workspace owns the
final `orc.yaml`, local edits, runtime state, and feature folders. Pack update
and uninstall are intentionally deferred.

## STATE.yaml

Every ticket has one. Agents update it as work progresses. `orc` reads it to
route work to the right agent.

```yaml
schema_version: 1
ticket: STORY-123
slug: STORY-123-add-login
status: active
workflow: default:standard

stage:
  worker: default:bob
  name: default:develop

next_action:
  worker: default:bob
  prompt: Implement the login feature per SPEC.md and PLAN.md.
  cwd: worktrees/my-app/STORY-123-add-login

runtime:
  mux:                          # exact target written by new launches
    backend: herdr              # tmux or herdr
    workspace: w-01K2ABC
    tab: t-01K2DEF
    pane: p-01K2GHI

  jit:                          # present while a jit task is running, absent otherwise
    worker: default:zach
    task: "check the auth middleware handles token expiry"
    started_at: "2026-06-01T13:45:00-05:00"

history:
  - at: "2026-05-28 09:00"
    stage: default:intake
    worker: default:fred
    result: ticket context loaded, SPEC.md and PLAN.md written
  - at: "2026-05-29 14:22"
    stage: default:develop
    worker: default:bob
    result: paused — need product decision on refresh token TTL
  - at: "2026-05-30 09:10"
    stage: default:develop
    worker: default:bob
    result: resumed after human clarified TTL should be 7 days
```

### Status values

| Status | Meaning | Set by |
|--------|---------|--------|
| `pending` | Session not yet started for the current stage | `orc work`, `orc mark <ticket> next` |
| `ready` | Human-set: cleared for the next session | human |
| `active` | Agent is actively working | `orc mark <ticket> start`, `orc mark <ticket> resume` |
| `paused` | Human needed — input, approval, or external blocker | `orc mark <ticket> pause` |
| `done` | All stages complete, or explicitly closed | `orc mark <ticket> next` (final stage) or `orc mark <ticket> done` |
| `archived` | Feature folder moved to `_archive/` | `orc archive` |

`runtime.mux` is the source of truth for new multiplexer launches. Its backend,
workspace, tab, and pane values are opaque identities returned by the selected
backend; labels are never reconstructed into IDs. `orc attach`, inventory,
status, dashboard, and archive cleanup use that recorded target.

Legacy `runtime.tmux.session` and `runtime.tmux.pane` remain readable and are
projected as a tmux target in memory. Loading old state does not rewrite it.
Older tickets without a pane continue to use their stage window; if that window
later has multiple panes, Orc requires exactly one pane marked
`@orc_agent=1` instead of guessing.

The structured agent-control surface is available for Herdr- and tmux-backed
tickets:

```bash
orc ctl agent state --ticket ORC-9
orc ctl agent prompt --ticket ORC-9 "Review this diff" --wait --timeout 120s
orc ctl agent wait --ticket ORC-9 --until blocked --timeout 120s
```

All commands target the exact recorded pane and emit JSON. `agent state` reads
the current recognized lifecycle without waiting or changing focus. `--until`
may be repeated with `idle`, `working`, `blocked`, `done`, or `unknown`; without
it, each backend waits for its settled defaults (`idle`, `done`, or `blocked`).
Tmux lifecycle state comes only from installed provider hooks, never screen
text. Tmux prompts are limited to 64 KiB of valid UTF-8, reject unsafe control
data, require the target application to enable bracketed paste, and never place
the prompt in a shell command or process argument. Waiting prompts preserve the
stable `agent_prompt_stalled` outcome when no new authoritative lifecycle
sequence follows submission.

`runtime.jit` is present only while a one-off JIT task is open. Finish the task
with `orc mark <ticket> jit "<summary>"`; that records a history entry and clears
the JIT runtime block.

State writes use `STATE.yaml.lock` with atomic temp-file replacement. If an orc
process dies mid-write, the next state write can recover dead-PID locks and old
malformed locks automatically. `orc doctor` reports any lock files it finds so
you can tell whether a live process is holding state or a stale lock will be
recovered on the next write; `orc doctor --fix` removes the stale ones
immediately without waiting for a write.

## Workers

Markdown files with YAML frontmatter. The frontmatter defines who the worker is
and how to launch them. The body gives the agent behavioral guidance.

```markdown
---
id: default:bob
name: Bob the Developer
engine: codex
model: gpt-5.5
args:
  reasoning_effort: high
  service_tier: medium
---

Implements features, opens PRs, and repairs CI failures.
```

`orc.yaml` declares the default worker per stage via `worker: <id>` in each stage entry. `orc next` looks up that worker, builds the prompt, and launches it.

**What goes into the prompt:**

Every launch gets a preamble pointing the agent at `AGENTS.md` and `ORC.md`, followed by the task prompt from `STATE.yaml`'s `next_action` field (or a generated one pointing at `features/<slug>/STATE.yaml` and `stages/<stage>.md`), and a closing instruction with the exact `orc mark` command to run when done — including whether the next advance is `auto` or `manual`.

When relaunching a paused or interrupted session, `orc next` builds a richer recovery prompt that also includes: recent history entries (what each prior stage did and who ran it), any partial output files already written to the current stage folder, and a checklist of key context files to read — `TICKET.md`, `SPEC.md`, `DECISIONS.md`, and the stage doc.

When a repo referenced by the feature has `agent_hints`, the launch prompt
includes those hints. When the current stage has `required_artifacts`, the prompt
lists the files the agent should keep current before completing the stage.
Outside the launch prompt, `orc artifacts <ticket>` shows the same readiness
check for the current stage, and `orc artifacts <ticket> --all --json` exposes
the workflow-wide checklist for automation.

This means no agent ever starts cold. The prompt is a complete handoff: what the ticket is, where things stand, what this stage needs to produce, which repo conventions apply, and exactly what command ends the session. The agent reads the files, does the work, runs the command — and the next agent gets the same treatment.

**Worker resolution order:**

1. `--worker <id>` flag on `orc next` — one-off override
2. `stage.worker` in `STATE.yaml` — set by a previous `orc mark <ticket> next --worker`
3. `worker:` for the current stage in `orc.yaml`

If no worker is found at any step, `orc next` exits with a clear error pointing to `orc.yaml`. Use `--dry` to preview the full launch command before running it.
