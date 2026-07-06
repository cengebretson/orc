# ORC.md — Agent State Contract

<!-- This file defines the orc state contract. Do not add team conventions or
     custom instructions here — put those in AGENTS.md under Team Conventions.
     Editing this file may break agent session protocol and state transitions. -->

Read this file at the start of every session.

Also read:
- `RULES.md` — what requires human approval before acting
- `AGENTS.md` — routing, tool policy, and repo commands

---

## Session Protocol

**Start every session:**
```
orc status <ticket> --json
```
Then mark the ticket active based on its current status:
- `pending` or `ready` → `orc mark <ticket> start`
- `paused` → `orc mark <ticket> resume`

Read the current stage file. Namespaced stage IDs map to nested paths:
`default:develop` is `stages/default/develop.md`.

**End every session with exactly one of:**
```
orc mark <ticket> next --result "<what was done>"                          # stage complete
orc mark <ticket> next --stage <stage> --worker <worker> --result "<what was done>" # explicit jump or override
orc mark <ticket> pause "<what you need from the human or what is blocking>"  # human needed
orc mark <ticket> done --result "<what was done>"                             # final stage
```
Never end a session without updating state. Never hand-edit STATE.yaml directly.

**Before any human interaction:**
Run `orc mark <ticket> pause "<what you need>"` before asking a human for input,
approval, or a decision. State must reflect reality even if the session ends
before the human responds. Do not ask, post, or request anything from a human
until STATE.yaml shows `paused`.

---

## Resource Names and Aliases

Orc resources use canonical IDs:

- Workflows: `<pack>:<workflow>`
- Stages: `<pack>:<stage>`
- Workers: `<pack>:<worker>`

`orc.yaml` may define aliases such as `develop` for `default:develop`. Orc
commands may accept either aliases or canonical IDs, but state and validation may
store canonical IDs. Always resolve files through the namespaced runtime paths:

- `stages/default/develop.md` for stage `default:develop`
- `workers/default/bob.md` for worker `default:bob`

Do not create root-level runnable files like `workers/bob.md` or
`stages/develop.md`.

---

## orc mark — Command Reference

```
orc mark <ticket> start                                               # begin fresh session (pending or ready)
orc mark <ticket> resume                                              # continue a paused session
orc mark <ticket> next --result "<what was done>"                     # stage complete, move to next
orc mark <ticket> next --stage <stage> --worker <worker>             # jump to a specific stage
orc mark <ticket> pause "<what you need or what is blocking>"        # human needed (input, approval, or blocker)
orc mark <ticket> done [--result "<what was done>"]                  # all stages complete
```

Use `start` for a fresh session on a `pending` or `ready` ticket.
Use `resume` to pick up a `paused` ticket — it clears the human-directed next action so you can write fresh context.
Use `next` when the stage exit criteria are met. If no stages remain, status is automatically set to `done`.
Use `pause` when you need a human decision, approval, information, or when an external condition prevents progress.
Use `done` to explicitly close active, ready, or paused work.

Transition guards:
- `start` is allowed only from `pending` or `ready`.
- `resume` is allowed only from `paused`.
- `next` is rejected while a ticket is still `pending`; start the session first.
- `next --stage` must name a configured workflow or loop stage, by alias or canonical ID.
- `next --worker` must name a worker, by alias or canonical ID, whose file exists under `workers/<pack>/`.
- `done` is rejected from `pending`.
- Invalid `orc.yaml` blocks `next`.

---

## Status Values

| Status | Meaning |
|--------|---------|
| `pending` | Session not yet started for the current stage — set by `orc work` and after each `orc mark next` |
| `ready` | Human-set: stage complete and cleared for the next session |
| `active` | Agent is actively working |
| `paused` | Human needed — input, approval, or external blocker |
| `done` | All stages complete (or explicitly closed) |
| `archived` | Feature folder moved to `_archive/` |

Use `orc mark <ticket> pause "<reason>"` for all cases where a human needs to act. The reason captures the details.

---

## STATE.yaml Update Rules

`STATE.yaml` is the durable state contract for the feature. Missing
`schema_version` means legacy v1.

| Field | Owner | Notes |
|-------|-------|-------|
| `schema_version` | `orc-owned` | State file contract version. New files use v1. |
| `ticket` | `orc-owned` | Stable ticket identifier. |
| `slug` | `orc-owned` | Feature folder slug. |
| `status` | `orc-owned` / `agent-writable` | Agents update this through `orc mark`. |
| `workflow` | `orc-owned` | Workflow selected when the feature is created. |
| `stage` | `orc-owned` | Current stage name and assigned worker. Change through `orc mark next`. |
| `stage_counts` | `orc-owned` | Retry and loop counts maintained by `orc`. |
| `runtime` | `orc-owned` | Runtime handles such as tmux session or active JIT task. |
| `repos` | `orc-owned` / `agent-writable` | Repo main paths, worktrees, and branches used for this feature. |
| `inputs` | `human-editable` / `agent-writable` | Context available to the current stage. |
| `outputs` | `agent-writable` | Required and completed stage outputs. |
| `next_action` | `agent-writable` | Who should act next, what they should do, and where commands should run. |
| `history` | `orc-owned` | Append-only summary of starts, transitions, pauses, and completions. |

`orc mark` writes a history entry automatically for every transition (start, resume, next, pause, done). Do not write history entries manually.

Agents are responsible for keeping these fields current as work progresses:

- `next_action` — set the worker, prompt, and cwd for whoever picks up next
- `repos` — record worktree path and branch when created or changed

`stage.name` and `stage.worker` are updated by `orc mark next`. Do not hand-edit them.

### STATE.yaml.lock

`orc` creates `STATE.yaml.lock` while it is writing state. Do not edit
`STATE.yaml` while the lock exists. If an `orc` command times out waiting for the
lock, run `orc doctor` and check whether the recorded PID is still active.

Locks with a dead PID, unreadable PID, or old timestamp are treated as stale and
may be removed by `orc` during the next state update. Active locks mean another
`orc` process is still writing state.

---

## Worktrees

Agents may create Git worktrees when a stage requires repository changes. Worktrees
are created by agents, but they must be tracked in `STATE.yaml` so later stages
and `orc archive` know what happened.

Create worktrees under the workspace:

```
worktrees/<repo-name>/<ticket-slug>/
```

Use repo names from `orc.yaml`. When you create or use a worktree, update
`STATE.yaml`:

```yaml
repos:
  <repo-name>:
    main: /absolute/path/to/main/repo
    worktree: worktrees/<repo-name>/<ticket-slug>
    branch: <branch-name>
```

Rules:

- Use the worktree as `cwd` for repo-specific package, test, and git commands.
- Set `next_action.cwd` to the worktree path when the next agent should continue there.
- Record the branch and worktree path before ending the session.
- Do not manually delete worktrees during feature work; `orc archive` handles cleanup.
- If the correct repo, branch, or worktree path is unclear, use `orc mark ... pause` and ask.

---

## Feature Folder

Every ticket has a context pack at `features/<ticket-slug>/`:

| File | Purpose |
|------|---------|
| `STATE.yaml` | Durable state — status, stage, worker, next action, history |
| `TICKET.md` | Ticket description and acceptance criteria |
| `SPEC.md` | Context, scope, constraints, open questions |
| `PLAN.md` | Implementation approach, repo context, and steps |
| `DECISIONS.md` | Non-obvious choices — what, why, alternatives rejected |

Read `STATE.yaml` and `TICKET.md` at the start of every session. Read `SPEC.md` and `PLAN.md` before any implementation work.

---

## Stage Handoff

The feature folder is the handoff medium between stages. Read previous stage outputs before starting work. If a required input is missing, `orc mark ... pause` — do not proceed.

Each stage writes its outputs to the paths declared in its stage instructions and
any `required_artifacts` configured for the stage in `orc.yaml`. `orc validate`
warns when these files are missing or empty. If `settings.artifact_policy` is
`block`, `orc mark <ticket> next` refuses to advance until core docs and the
current stage's required artifacts are ready.

These are handoff folders, not canonical resource IDs. The default pack uses
friendly folders such as `develop/`, `code-review/`, and `pr-open/`.

Do not create feature output folders named after canonical IDs such as
`default:develop/` unless the stage instructions explicitly require it.

| Path | Written by | Read by |
|------|-----------|---------|
| `TICKET.md` | intake | all stages |
| `SPEC.md` | intake | develop, code-review |
| `PLAN.md` | intake | develop, code-review, pr-open, qa-automation |
| `DECISIONS.md` | any stage | any stage |
| `develop/HANDOFF.md` | develop | code-review, pr-open, qa-automation |
| `code-review/REVIEW.md` | code-review | develop, pr-open |
| `pr-open/PR.md` | pr-open | pr-repair, qa-automation, human |
| `qa-automation/PLAN.md` | qa-automation | qa-automation (next session) |
| `qa-automation/RUNS.md` | qa-automation | qa-automation, human |
| `qa-automation/RESULT.md` | qa-automation | human, archive |

---

## Recording Decisions

When you make a non-obvious choice, write it to `features/<ticket-slug>/DECISIONS.md` at the moment of the decision:

```
## <short title>
**Decision:** <what>
**Reason:** <why — constraints, tradeoffs, context>
**Alternatives:** <what else was considered and why rejected>
```

One entry per decision. Do not batch at end of session.
