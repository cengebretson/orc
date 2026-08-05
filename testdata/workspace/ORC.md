# ORC.md — Agent State Contract

<!-- This file defines orc's durable state contract. Put team conventions and
     repository commands in AGENTS.md, and approval policy in RULES.md. -->

Read this file at the start of every ticket session. Also read:

- `RULES.md` for actions that require human approval
- `AGENTS.md` for shared workspace and repository command conventions
- the current stage file under `stages/<pack>/`

`RULES.md` is authoritative for permission. Stage files, worker files, launch
prompts, and operator phrases may request an outcome, but they do not authorize
shared, external, destructive, or difficult-to-reverse actions such as a Git
push. Follow configured workspace exceptions when they explicitly allow
automation; otherwise pause with the exact proposed action.

## Session Protocol

1. Inspect durable state: `orc status <ticket> --json`.
2. Read `TICKET.md` and the current stage file. Before implementation, also read
   `SPEC.md`, `PLAN.md`, and relevant prior-stage outputs.
3. Enter the session:
   - `pending` or `ready`: `orc mark <ticket> start`
   - `paused`: `orc mark <ticket> resume`
   - `active`: continue only if this is the same active session; otherwise pause
     and resolve the ownership conflict
4. Do the stage work and keep the feature context current.
5. End with exactly one durable transition:

```text
orc mark <ticket> next --result "<what was done>"
orc mark <ticket> next --stage <stage> --worker <worker> --result "<what was done>"
orc mark <ticket> pause "<what the human must decide or what is blocking>"
orc mark <ticket> done --result "<what was done>"
```

Never leave an active session without recording its outcome. Before asking a
human for input, approval, or a decision, pause the ticket with the specific
request so durable state reflects who must act next.

Use `next` only when the stage exit criteria are met. If required artifacts are
missing, pause instead of using `--force`; that override is for humans. Use
`done` only when the workflow is genuinely complete.

## Operator Phrases

The phrases below are messages an operator may send to an agent. They are
protocol shorthand, not necessarily literal shell commands. Resolve the current
ticket from the launch prompt and durable state, then use the real Orc commands
required by this contract.

| Operator phrase | Underlying command or action | Agent behavior |
|---|---|---|
| `orc status` | `orc status <ticket> --json` | Report the current stage, progress, blockers, and next action without changing state. |
| `orc check` | `orc artifacts <ticket>`, then the current stage's validation commands | Report artifact or validation failures without changing lifecycle state. |
| `orc handoff` | Update feature artifacts and allowed `STATE.yaml` context fields, then run `orc doctor <ticket>` | Persist progress, decisions, validation, and the next action; remain active unless another phrase requests a transition. |
| `orc pause: <reason>` | `orc mark <ticket> pause "<reason>"` | Persist the precise blocker or human decision needed, then stop work. |
| `orc resume` | `orc mark <ticket> resume`, then `orc status <ticket> --json` | Reload durable state and continue the current stage. |
| `orc done` | The exact end transition supplied in the launch prompt or current stage instructions | Verify exit criteria, write the actual result summary, and perform the correct `next`, `pause`, or `done` transition. |
| `orc help` | No lifecycle command | Briefly list these phrases and show the exact end transition for the current session. |

`orc done` does not mean “mark the whole workflow done” when the current stage
must advance or pause; the exact end transition from the launch prompt or stage
instructions remains authoritative. `orc archive` is not conversational
shorthand. Run it only as an explicit archival action after completion because
it may terminate the recorded multiplexer session.

## State Ownership

`STATE.yaml` is the durable handoff between sessions. Orc owns lifecycle state;
agents own the work context that orc cannot infer.

Never hand-edit these orc-owned fields:

- `schema_version`, `ticket`, `slug`, `status`, `workflow`
- `stage`, `stage_counts`, `runtime`, `history`

Change lifecycle state only with `orc mark`, `orc jit`, `orc archive`, and other
orc commands. In particular, never add history entries or alter JIT/runtime
handles manually.

Agents and humans may directly update only these context fields when the work
requires it:

- `repos`: repository main paths, worktrees, and branches
- `inputs`: new context supplied for the current work
- `outputs`: required and completed stage outputs
- `next_action`: who acts next, what they should do, and the correct `cwd`

Do not edit `STATE.yaml` while `STATE.yaml.lock` exists. Keep direct edits
minimal and preserve all other fields. After an edit, run
`orc doctor <ticket>` to validate the state file.

`orc mark` records transition history automatically. Important statuses are:

| Status | Meaning |
|---|---|
| `pending` | Current stage has not started |
| `ready` | Work is cleared to start; accepted as a startable compatibility state |
| `active` | An agent session is working |
| `paused` | A human action or external condition is required |
| `done` | Workflow is complete or explicitly closed |
| `archived` | Feature folder has been archived |

If a command times out on the state lock, run `orc doctor <ticket>` and inspect
whether the recorded process is still active. Do not remove an active lock. Orc
can recover locks whose process is dead or whose metadata is stale.

## Resource IDs

Canonical resource IDs are namespaced:

- workflows: `<pack>:<workflow>`
- stages: `<pack>:<stage>`
- workers: `<pack>:<worker>`

Commands may accept aliases configured in `orc.yaml`, but state may store the
canonical ID. Resolve runnable files through namespaced paths: for example,
`default:develop` maps to `stages/default/develop.md`. Do not create runnable
root files such as `stages/develop.md` or `workers/bob.md`.

`next --stage` must name a configured workflow or loop stage. `next --worker`
must name a configured worker whose file exists. Invalid configuration blocks
the transition.

## Feature Context and Handoffs

Each ticket has a context pack at `features/<ticket-slug>/`. Core files are:

| Path | Purpose |
|---|---|
| `STATE.yaml` | Durable lifecycle and handoff state |
| `TICKET.md` | Ticket description and acceptance criteria |
| `SPEC.md` | Scope, constraints, and open questions |
| `PLAN.md` | Implementation approach and repository context |
| `DECISIONS.md` | Non-obvious choices and rejected alternatives |

The feature folder is the handoff medium. Stage instructions and
`required_artifacts` in `orc.yaml` are authoritative for stage-specific output
paths. Read relevant prior outputs before starting. If a required input is
missing, pause with a precise request.

The default pack uses friendly output folders such as `develop/`,
`code-review/`, `pr-open/`, `pr-repair/`, and `qa-automation/`. These are paths,
not canonical resource IDs; do not create folders such as `default:develop/`.

With `settings.artifact_policy: block`, `orc mark <ticket> next` rejects missing,
empty, or unchanged template artifacts. `orc artifacts <ticket>` reports
artifact problems. Agents do not bypass these checks with `--force`.

## Repository Routing

Repository routing is a workflow-independent precondition for repository work.
The default intake stage resolves it early, but custom workflows are not
required to contain an intake stage.

Before a stage reads or changes repository code:

1. Use the repositories already recorded in `STATE.yaml.repos` when they fully
   cover the stage's work.
2. If that selection is empty or incomplete, resolve it from `orc.yaml`:
   - honor explicit configured repository names in trusted ticket context;
   - match exact ticket labels or components against `routing` rules;
   - otherwise compare the ticket scope with repository `purpose` and
     `agent_hints`.
3. Persist every selected repository as a key in the agent-writable
   `STATE.yaml.repos` field before running repository commands.
4. If multiple routing rules match or the fallback remains ambiguous, pause
   with the routing decision needed. Do not silently merge rules or guess.

One routing rule may explicitly select multiple repositories. Ticket ID prefixes
identify ticket namespaces, not repository ownership, and must not be used as a
repository selector. Workflow selection is independent of repository routing.

## Worktrees

When a stage requires repository changes, create worktrees under:

```text
worktrees/<repo-name>/<ticket-slug>/
```

Use repository names from `orc.yaml`. Immediately record the main repository,
worktree, and branch in the agent-writable `repos` field:

```yaml
repos:
  <repo-name>:
    main: /absolute/path/to/main/repo
    worktree: worktrees/<repo-name>/<ticket-slug>
    branch: <branch-name>
```

Run repository-specific commands from the worktree. Set `next_action.cwd` to the
worktree when the next session should continue there. Do not manually delete a
tracked worktree during feature work; `orc archive` handles cleanup. If the
repository, branch, or path is unclear, pause and ask.

## Decisions

Record a non-obvious choice in `DECISIONS.md` when it is made:

```markdown
## <short title>
**Decision:** <what>
**Reason:** <constraints and tradeoffs>
**Alternatives:** <what was rejected and why>
```

One entry per decision; do not defer decision logging until session end.
