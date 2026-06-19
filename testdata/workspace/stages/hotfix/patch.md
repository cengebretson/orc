# Stage: patch

> Before starting: read `ORC.md` for state update rules and error handling.

## Purpose

Apply a targeted fix to the production codebase with minimal blast radius.
Runs after `intake` confirms the issue and scope.

## Steps

**Owner:** developer agent  
**Inputs:** `TICKET.md`, `SPEC.md`, repo worktree (see `STATE.yaml` → `next_action.cwd`)  
**Outputs:** `patch/HANDOFF.md`, committed fix on hotfix branch

1. Read `TICKET.md` and `SPEC.md` to understand the exact regression.
2. Reproduce the failure locally before touching code.
3. Apply the minimal fix. Do not refactor surrounding code.
4. Run the relevant test suite. All existing tests must pass.
5. Write `patch/HANDOFF.md` with:
   - Root cause
   - Files changed
   - Tests added or updated
   - Verification steps
6. When complete, run:
   ```
   orc mark TICKET wait "patch ready for deploy — <one-line summary>"
   ```

## Exit Criteria

- Regression is fixed and verified locally
- No unrelated changes in the diff
- `patch/HANDOFF.md` written
