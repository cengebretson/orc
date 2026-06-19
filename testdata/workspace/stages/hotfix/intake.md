# Stage: intake

> Before starting: read `ORC.md` for state update rules and error handling.

## Purpose

Load ticket context and create the feature folder. This is the required first
workflow for any ticket-driven work — nothing downstream runs until intake completes.

## Trigger

```
orc work <ticket>
```

## Steps

**Owner:** intake agent  
**Inputs:** Ticket ID  
**Outputs:** `TICKET.md`, `SPEC.md`, `PLAN.md`

1. Read `ROUTER.md` — the **Ticket System** section tells you where tickets
   live, how to fetch them, and any auth requirements. Use that.
2. Fetch the ticket from the source system described in `ROUTER.md`.
3. If the ticket cannot be found, run `orc mark <ticket> pause "<explanation>"` and stop.
4. Populate `TICKET.md` with the ticket summary, description, and acceptance criteria.
5. Draft `SPEC.md` with context, scope, and open questions.
6. Draft `PLAN.md` with an initial approach and steps.

## Exit Criteria

`TICKET.md`, `SPEC.md`, and `PLAN.md` are populated.

When done, run:
```
orc mark <ticket> next --stage develop --worker <worker-id> --result "Intake complete"
```

## Error Handling

If the ticket cannot be found or fetched:
- Run `orc mark <ticket> pause "<description of what failed and what to check>"`
- Do not populate files with placeholder content
- Stop — a human must resolve the issue before work continues
