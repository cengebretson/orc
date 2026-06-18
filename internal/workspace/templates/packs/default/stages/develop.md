# Stage: develop

> Before starting: read `ORC.md` for state update rules and error handling.

## Purpose

Implement the feature in a repo worktree and prepare it for code review.
Runs after the `intake` stage completes, and again after `code-review` sends
work back for rework.

## Rework Detection

Before starting, check whether `code-review/REVIEW.md` exists in the feature folder.

**If `code-review/REVIEW.md` does not exist** — this is the initial implementation pass.
Follow the steps below and end with `orc mark ... pause` for human approval before code review.

**If `code-review/REVIEW.md` exists with `verdict: needs-changes` or `verdict: blocked`** — this is a
rework pass in response to review feedback. Read the findings carefully, address every
item marked `[bug]`, `[spec]`, or `[risk]` before anything else. `[style]` and `[minor]`
items should be fixed if straightforward. When done, advance directly to code-review
without waiting for human approval — the reviewer will verify the fixes.

## Steps

**Owner:** developer agent  
**Inputs:** `PLAN.md`, `SPEC.md`, repo worktree (see `STATE.yaml` → `next_action.cwd`),  
`code-review/REVIEW.md` (rework pass only)  
**Outputs:** `develop/HANDOFF.md`, committed code on feature branch

1. Read `SPEC.md` and `PLAN.md` for context.
2. If `PLAN.md` has a `Repo Context` section with relevant docs, read those
   docs before implementation.
3. If rework pass: read `code-review/REVIEW.md` and address all findings before proceeding.
4. Implement (or fix) the feature in the repo worktree.
5. Write and run local tests for changed files.
6. Write (or update) `develop/HANDOFF.md` with an implementation summary, domain
   or repo-context impact, test instructions, and known risks.
7. Commit all changes to the feature branch.

## Exit Criteria

Code is committed, `develop/HANDOFF.md` is written, and local tests pass.

**Initial pass** — run:
```
orc mark <ticket> pause "Implementation complete — human review before code review"
```

**Rework pass** — run:
```
orc mark <ticket> next --stage code-review --worker <worker-id> --result "Rework complete — addressed review findings"
```
