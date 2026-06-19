# ORC.md — Agent State Contract

Read this file at the start of every session.

Also read:
- `RULES.md` — what requires human approval before acting
- `AGENTS.md` — routing, tool policy, and repo commands

---

## Session Protocol

**Start every session:**
```
orc mark <ticket> start
orc status <ticket> --json
```
Read `stages/<pack>/<stage>.md` for namespaced stage IDs. For example,
`default:develop` is `stages/default/develop.md`.

**End every session with exactly one of:**
```
orc mark <ticket> next --worker <next-worker> --result "<what was done>"   # stage complete
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

## orc mark — Command Reference

```
orc mark <ticket> start                                               # begin session, sets active
orc mark <ticket> next --result "<what was done>"                     # stage complete, move to next
orc mark <ticket> next --stage <name> --worker <id>                  # jump to a specific stage
orc mark <ticket> pause "<what you need or what is blocking>"        # human needed (input, approval, or blocker)
orc mark <ticket> done [--result "<what was done>"]                  # all stages complete
```

Use `next` when the stage exit criteria are met. If no stages remain, status is automatically set to `done`.
Use `pause` when you need a human decision, approval, information, or when an external condition prevents progress.
Use `done` to explicitly close active, ready, or paused work.

Transition guards:
- `start` is allowed only from `pending`, `ready`, or `paused`.
- `next` is rejected while a ticket is still `pending`; start the session first.
- `next --stage` must name a configured workflow or loop stage.
- `next --worker` must name a worker from `workers/`.
- `done` is rejected from `pending`.
- Invalid `orc.yaml` blocks `next`.

---

## Status Values

| Status | Meaning |
|--------|---------|
| `pending` | Feature created, intake not yet run |
| `ready` | Stage complete, queued for next agent |
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
| `stage` | `orc-owned` / `agent-writable` | Current stage name and assigned worker. Change through `orc mark next`. |
| `stage_counts` | `orc-owned` | Retry and loop counts maintained by `orc`. |
| `runtime` | `orc-owned` | Runtime handles such as tmux session or active JIT task. |
| `repos` | `orc-owned` / `agent-writable` | Repo main paths, worktrees, and branches used for this feature. |
| `inputs` | `human-editable` / `agent-writable` | Context available to the current stage. |
| `outputs` | `agent-writable` | Required and completed stage outputs. |
| `next_action` | `agent-writable` | Who should act next, what they should do, and where commands should run. |
| `history` | `agent-writable` | Append-only summary of starts, transitions, pauses, and completions. |

Write a history entry for every stage transition, block, or wait:

```yaml
- at: <RFC3339>
  stage: <stage name>
  worker: <worker id or "human">
  result: <one line>
```

Also update `stage.name`, `stage.worker`, `next_action`, and `repos` whenever those change.

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
| `STATE.yaml` | Durable state — status, stage, owner, next action, history |
| `TICKET.md` | Ticket description and acceptance criteria |
| `SPEC.md` | Context, scope, constraints, open questions |
| `PLAN.md` | Implementation approach and steps |
| `DECISIONS.md` | Non-obvious choices — what, why, alternatives rejected |

Read `STATE.yaml` and `TICKET.md` at the start of every session. Read `SPEC.md` and `PLAN.md` before any implementation work.

---

## Stage Handoff

The feature folder is the handoff medium between stages. Read previous stage outputs before starting work. If a required input is missing, `orc mark ... pause` — do not proceed.

Each stage writes its outputs to a subfolder matching its name: `<stage-name>/`. This makes provenance unambiguous — if you need to find what `develop` produced, look in `develop/`.

| Path | Written by | Read by |
|------|-----------|---------|
| `TICKET.md` | intake | all stages |
| `SPEC.md` | intake | develop, code-review |
| `PLAN.md` | intake | develop |
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
