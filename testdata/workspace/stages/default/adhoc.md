# Stage: ad hoc

> Before starting: read `AGENTS.md` and `ORC.md` for repository and state rules.

## Purpose

Complete one standalone local task created by `orc run`. The original
instruction is preserved in `TICKET.md` and in the launch prompt. This workflow
has no intake, review, PR, or QA stages unless the task itself calls for them.

## Steps

1. Read `TICKET.md` and the current `STATE.yaml`.
2. Inspect the relevant repository context and make a short plan appropriate to
   the task.
3. Complete the requested work and run proportionate validation.
4. Preserve useful decisions or notes in the feature folder.

## Exit Criteria

The requested work and relevant validation are complete. Run:

```sh
orc mark <ticket> done --result "<summary of what was done>"
```
