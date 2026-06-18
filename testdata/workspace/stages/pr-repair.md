# Stage: pr-repair

> Before starting: read `ORC.md` for state update rules and error handling.

## Purpose

Repair a PR that has CI failures, review feedback, or merge conflicts.

## Steps

**Owner:** developer agent  
**Inputs:** `pr-open/PR.md`, CI output, review comments  
**Outputs:** Fixed commits, updated `pr-open/PR.md`

1. Read `pr-open/PR.md` for PR URL and current status.
2. Identify the failure: CI, review feedback, or conflict.
3. Apply fixes in the app worktree.
4. Run local validation for the changed files.
5. Push fixes and check CI.
6. Update `pr-open/PR.md` with new status.

## Cost Note

Prefer bob-developer for lint, type, selector, test, and CI fixes. Escalate to
ada-architect only for design-level problems.

## Exit Criteria

CI is green and all blocking review comments are resolved.

When done, run:
```
orc mark <ticket> next --stage pr-open --worker <worker-id> --result "PR repaired — CI green"
```

If the issue cannot be resolved:
```
orc mark <ticket> block "<what is blocking and what a human needs to decide>"
```
