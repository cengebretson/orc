# Stage: pr-open

> Before starting: read `ORC.md` for state transitions and `RULES.md` for
> external-action permissions. If push or PR automation is not authorized,
> pause with the exact proposed action instead of performing it.

## Purpose

Open a pull request after implementation is complete. Runs final validation,
writes the PR description, opens the PR, and hands off for human review.

## Steps

**Owner:** developer agent  
**Inputs:** `develop/HANDOFF.md`, `code-review/REVIEW.md`, `TICKET.md`, feature worktree  
**Outputs:** Open PR, populated `pr-open/PR.md`

**Preflight**
1. Read `features/<ticket-slug>/STATE.yaml` for the active worktree and branch.
2. Run local validation: tests, lint, type checks for changed files only.
3. If checks fail, run `orc mark <ticket> block "<failure details>"` and stop — do not open a PR against a broken branch.
4. Ensure branch is rebased or merged against the base branch with no conflicts.
5. Push the branch to the remote.

**Open PR**
6. Read `TICKET.md` for the ticket summary and acceptance criteria.
7. Read `develop/HANDOFF.md` for the implementation summary.
8. Write a PR title: concise, under 70 characters, describes what changed.
9. Write a PR body: what changed and why (link ticket), how to test, migration or deployment notes.
10. Open the PR via the source control MCP server (see `TOOLS.md`).
11. Write the PR URL and status to `pr-open/PR.md`.

## Exit Criteria

PR is open and `pr-open/PR.md` has the URL.

When done, run:
```
orc mark <ticket> pause "PR open — waiting for human review"
```

If preflight checks fail:
- Run `orc mark <ticket> block "<specific failure and what needs fixing>"`
- Do not push or open a PR
