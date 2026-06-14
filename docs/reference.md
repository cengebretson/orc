# orc reference

Deep reference for the workspace `orc` scaffolds and the files it manages. For
the getting-started guide, command list, and concepts, see the
[README](../README.md). For workflow configuration, see
[workflows.md](workflows.md).

## Workspace layout

```
my-workspace/
  AGENTS.md          shared context and routing rules (Claude + Codex)
  CLAUDE.md          imports AGENTS.md (Claude entrypoint)
  ROUTER.md          which repo owns each task, worktree paths
  TOOLS.md           approved tools, MCP servers, external systems
  RULES.md           approval, state update, and cost rules
  SETUP.md           one-time setup — run with your agent after init
  .gitignore         excludes worktrees/

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
    # the `default` pack also adds persona workers:
    # fred-the-documentor.md (intake), bob-the-developer.md, zach-the-reviewer.md, …

  stages/
    intake.md        load ticket context — runs first for every ticket
    develop.md       implementation
    code-review.md   review implementation before opening PR
    pr-open.md       preflight checks, open PR, handoff for review
    pr-repair.md     fix CI failures, review feedback, conflicts
    qa-automation.md implement and run automated tests
    # provided by the installed pack (default shown); plain markdown —
    # no frontmatter, flow control lives in orc.yaml

  orc.yaml           workspace config — repos, workflows, loop stages, settings
  ORC.md             agent state contract — read at session start

  worktrees/         git worktrees for ticket branches (gitignored)
```

## Workspace files

The root files are the shared context every agent reads before starting work. Each has a distinct owner and purpose.

| File | Owner | Purpose |
|------|-------|---------|
| `AGENTS.md` | shared | Entry point for all agents — routing, session protocol, repo commands. Add team conventions at the bottom. |
| `CLAUDE.md` | orc | Imports `AGENTS.md`. Claude's entrypoint — do not edit. |
| `ORC.md` | orc | State contract — status values, `orc mark` commands, STATE.yaml rules. Do not add team conventions here. |
| `ROUTER.md` | user | Ticket system details, repo purposes, worktree layout. Fill in during setup. |
| `TOOLS.md` | user | Approved tools, MCP servers, CLI commands, external systems. Fill in during setup. |
| `RULES.md` | user | What requires human approval before agents act — PR gates, cost limits, destructive operations. |
| `SETUP.md` | orc | One-time setup guide. Run with your agent after `orc init` to configure repos, workers, and tool policy. |
| `orc.yaml` | user | Workflow config — repos, stage order, workers, loop stages, settings. |

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
  default_workflow: default
  auto_archive: false
  auto_tmux: false       # wrap every orc next launch in a tmux session automatically
  auto_next: false       # orc work immediately launches the first stage (same as --next)
  tui_refresh: 60        # dashboard auto-refresh interval in seconds
  theme: catppuccin-mocha

repos:
  - name: my-app
    path: ../my-app
    purpose: Application code, APIs, tests

workflows:
  default:
    description: General feature workflow — intake → develop → PR → QA
    stages:
      - name: intake
        worker: fred-documentor
        advance: auto
      - name: develop
        worker: bob-developer
        advance: manual
        loop:
          via: code-review
          worker: zach-reviewer
          max: 3
          on_max: pause
      - name: pr-open
        worker: bob-developer
        advance: manual
        loop:
          via: pr-repair
          worker: bob-developer
          max: 3
          on_max: pause
      - name: qa-automation
        worker: brian-qa
        advance: auto
```

`default_workflow` is used by `orc work <ticket>` when `--workflow` is omitted.
If it is not set, `orc work` returns an error. `advance: auto` tells agents to
run `orc mark <ticket> next` when a stage is complete; `advance: manual` tells agents to
run `orc mark <ticket> pause` so a human can review before continuing.

## STATE.yaml

Every ticket has one. Agents update it as work progresses. `orc` reads it to
route work to the right agent.

```yaml
schema_version: 1
ticket: STORY-123
slug: STORY-123-add-login
status: active
workflow: default

stage:
  worker: bob-developer
  name: develop

next_action:
  worker: bob-developer
  prompt: Implement the login feature per SPEC.md and PLAN.md.
  cwd: worktrees/my-app/STORY-123-add-login

runtime:
  tmux:                         # present when a tmux session is configured
    session: STORY-123-add-login

  jit:                          # present while a jit task is running, absent otherwise
    worker: zach-the-reviewer
    task: "check the auth middleware handles token expiry"
    started_at: "2026-06-01T13:45:00-05:00"

history:
  - at: "2026-05-28 09:00"
    stage: intake
    worker: fred-documentor
    result: ticket context loaded, SPEC.md and PLAN.md written
  - at: "2026-05-29 14:22"
    stage: develop
    worker: bob-developer
    result: paused — need product decision on refresh token TTL
  - at: "2026-05-30 09:10"
    stage: develop
    worker: bob-developer
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

`runtime.tmux.session` is the source of truth for tmux operations once present.
Older tickets without that field fall back to the feature slug. `orc attach`,
`orc status`, `orc tui`, and archive cleanup all use the recorded runtime session
so custom or restored session names continue to work.

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
id: bob-developer
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

This means no agent ever starts cold. The prompt is a complete handoff: what the ticket is, where things stand, what this stage needs to produce, and exactly what command ends the session. The agent reads the files, does the work, runs the command — and the next agent gets the same treatment.

**Worker resolution order:**

1. `--worker <id>` flag on `orc next` — one-off override
2. `stage.worker` in `STATE.yaml` — set by a previous `orc mark <ticket> next --worker`
3. `worker:` for the current stage in `orc.yaml`

If no worker is found at any step, `orc next` exits with a clear error pointing to `orc.yaml`. Use `--dry` to preview the full launch command before running it.
