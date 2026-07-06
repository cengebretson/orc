# ROUTER.md

## Session Root

Start every agent session at the workspace root — the directory containing this file.
Read the workspace docs here first, then navigate to the repo or worktree for code work.

Do not start a session inside a repo or worktree directly. Without the workspace root
as your starting point you are missing feature context, tool policy, routing rules,
and approval requirements.

**Context lives here. Code lives in repos. Commands run there.**

---

## Repos

Repos and their paths are declared in `orc.yaml`. Read that file to find repo names,
filesystem paths, and purposes. Use the path from `orc.yaml` as `cwd` when running
code commands — do not guess paths.

For ticket work, use the worktree path from `STATE.yaml` → `next_action.cwd`, not
the repo path directly.

---

## Routing

When deciding which repo owns a task, start with the ticket's feature folder:

1. Read `features/<ticket-slug>/STATE.yaml` — `next_action.cwd` is the worktree to use.
2. If there is no active ticket, check `orc.yaml` for repo purposes and pick the one
   that matches the work type.
3. Run code commands (tests, lint, build) from the worktree or repo path, not the
   workspace root.

---

## Worktrees

For ticket work, never edit a repo directly. Use an isolated git worktree so
branches stay clean and multiple tickets can run in parallel.

Worktrees live inside this workspace under `worktrees/<repo-name>/<ticket-slug>`.

If the repo in `orc.yaml` defines `worktree_setup`, use the setup command that
`orc next` prints for the ticket. That command is repo-specific and may copy
environment files, install dependencies, initialize submodules, or start a local
stack. It must create the worktree at the path `orc` passes.

If no repo-specific setup command is configured, create a worktree for a ticket:

```
git -C <repo-path> worktree add <workspace>/worktrees/<repo-name>/<ticket-slug> -b <branch>
```

The worktree path for the active ticket is always recorded in `STATE.yaml` under
`next_action.cwd`. Use that path — do not guess or construct it manually.

---

## Ticket System

**ROUTER.md is the source of truth for how to retrieve tickets in this workspace.**

Edit the section below to describe your ticket system — the agent reads this during
the intake stage to know where to fetch ticket data. Do not hardcode ticket system
details in stage files; keep them here so they apply to every ticket.

```
Ticket system: <e.g. Linear, GitHub Issues, Jira, local file>
Project / team key: <e.g. STORY, ENG, #my-repo>
How to retrieve a ticket: <e.g. gh issue view <number>, linear issue get <id>, cat tickets/<id>.md>
Authentication: <e.g. GITHUB_TOKEN env var, Linear API key in ~/.config/linear>
```

---

## Stages

Stage definitions live in `stages/<pack>/<stage>.md` for namespaced stage IDs
such as `default:develop`. Flow control (order, worker per stage, advance mode,
repair loops) is declared in `orc.yaml`.
