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
5. Identify the target repo or repos for the work. Use `orc.yaml`, the ticket,
   and `ROUTER.md`; if the correct repo is unclear, record the uncertainty in
   `SPEC.md` rather than guessing.
6. Check target repos for repo-local context that may affect the work, such as:
   - `AGENTS.md`, `CLAUDE.md`, `README.md`
   - `ARCHITECTURE.md`, `docs/architecture.md`
   - `docs/domains/REGISTRY.md`
   - `docs/domains/*/README.md`
7. Read only the context docs that are relevant to the ticket. Do not run broad
   analysis or update repo docs during intake.
8. Draft `SPEC.md` with context, scope, open questions, and any repo constraints.
9. Draft `PLAN.md` with an initial approach, steps, and a `Repo Context` section.

## Repo Context

In `PLAN.md`, include:

```markdown
## Repo Context

Relevant docs:
- `<path>` — why it matters

Impacted areas:
- `<area or domain>`

Risks:
- `<boundary, coupling, test, or docs risk>`

Missing context:
- `<important doc or owner that could not be found>`
```

If no relevant repo-local docs are found, write:

```markdown
## Repo Context

Relevant docs: none found
Impacted areas: inferred from ticket only
Risks: none identified from repo-local docs
Missing context: none
```

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
