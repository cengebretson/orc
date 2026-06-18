# Stage: qa-automation

> Before starting: read `ORC.md` for state update rules and error handling.

## Purpose

Plan and implement automated tests for a completed feature, then collect evidence.
Runs after the PR has been reviewed and the `pr-open` stage hands off here.

## Steps

**Owner:** default:brian agent  
**Inputs:** `develop/HANDOFF.md`, `TICKET.md`, QA repo worktree  
**Outputs:** `qa-automation/SOURCE_CONTEXT.md`, `qa-automation/PLAN.md`, `qa-automation/RUNS.md`, `qa-automation/RESULT.md`

**QA Planning**
1. Read `develop/HANDOFF.md` and `TICKET.md`.
2. Write `qa-automation/SOURCE_CONTEXT.md` with repo context the QA agent will need.
3. Draft `qa-automation/PLAN.md` with test cases, coverage goals, and tooling notes.

**Implementation**
4. Read `qa-automation/PLAN.md` and `qa-automation/SOURCE_CONTEXT.md`.
5. Implement test cases in the QA repo worktree.
6. Run tests and record results in `qa-automation/RUNS.md`.
7. Push and confirm CI passes.

**Evidence**
8. Read `qa-automation/RUNS.md` and CI results.
9. Write `qa-automation/RESULT.md` with pass/fail summary and coverage notes.
10. Update the ticket in the source system (see `TOOLS.md` for the MCP server to use).
11. Run `orc archive <ticket>` to close out the feature.

## Exit Criteria

`qa-automation/RESULT.md` is complete with passing status, ticket is updated, and
`orc archive` has been run.
