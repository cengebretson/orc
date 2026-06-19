# Stage: deploy

> Before starting: read `ORC.md` for state update rules and error handling.

## Purpose

Merge the hotfix branch and confirm the fix is live in production.

## Steps

**Owner:** developer agent  
**Inputs:** `patch/HANDOFF.md`, approved hotfix branch  
**Outputs:** `deploy/DEPLOY.md`

1. Read `patch/HANDOFF.md` to confirm the fix is ready.
2. Open a PR targeting the production branch (main or release).
3. Merge once CI passes — no additional review required for hotfix workflow.
4. Confirm the fix is deployed and the regression no longer reproduces.
5. Write `deploy/DEPLOY.md` with:
   - PR URL
   - Merge commit
   - Deploy timestamp
   - Confirmation the issue is resolved
6. When complete, run:
   ```
   orc mark TICKET advance --result "deployed — <one-line summary>"
   ```

## Exit Criteria

- Hotfix merged and deployed
- Regression confirmed resolved in production
- `deploy/DEPLOY.md` written
