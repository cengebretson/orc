# features/

Each subfolder is a durable context pack for one ticket. State survives session
changes, agent switches, and restarts — everything needed to pick up where work
left off is in these files.

Start a new ticket with:

```
orc work TICKET-0000
```

## Structure

```
features/
  _template/            copied for each new ticket by orc work
  _archive/             completed features moved here by orc archive
  TICKET-0000-slug/
    STATE.yaml          current stage, worker, status, next action
    TICKET.md           ticket description and acceptance criteria
    SPEC.md             context, scope, and open questions
    PLAN.md             implementation approach, repo context, and steps
    DECISIONS.md        significant decisions and rationale
    develop/            outputs written by the develop stage
      HANDOFF.md        implementation summary and known risks
    code-review/        outputs written by the code-review stage
      REVIEW.md         findings and verdict
    pr-open/            outputs written by the pr-open stage
      PR.md             PR URL and status
    qa-automation/      outputs written by the qa-automation stage
      SOURCE_CONTEXT.md repo context for the QA agent
      PLAN.md           test cases and coverage plan
      RUNS.md           test run history
      RESULT.md         final result and evidence
```

Each stage writes its outputs to a subfolder matching its name. Stages create their
own subfolder — nothing is pre-created in the template.
