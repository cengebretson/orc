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

1. Read `TOOLS.md` for the verified ticket retrieval method and fetch the ticket.
2. If the ticket cannot be found, run `orc mark <ticket> pause "<explanation>"` and stop.
3. Populate `TICKET.md` with the ticket summary, description, and acceptance criteria.
4. Resolve the repository-routing precondition from `ORC.md` proactively by
   identifying every target repository:
   - honor explicit repository names in trusted ticket metadata;
   - match exact ticket labels/components against `orc.yaml` routing rules;
   - otherwise compare ticket scope with each repository's `purpose` and
     `agent_hints`.
   A routing rule may intentionally select multiple repositories. Record each
   selected repository as a key in `STATE.yaml.repos`. If multiple rules match,
   or the fallback is still ambiguous, record the uncertainty in `SPEC.md` and
   pause rather than unioning rules or guessing.
5. Check target repos for repo-local context that may affect the work, such as:
   - `AGENTS.md`, `CLAUDE.md`, `README.md`
   - `ARCHITECTURE.md`, `docs/architecture.md`
   - `docs/domains/REGISTRY.md`
   - `docs/domains/*/README.md`
6. Read only the context docs that are relevant to the ticket. Do not run broad
   analysis or update repo docs during intake.
7. Draft `SPEC.md` with context, scope, open questions, and any repo constraints.
8. Draft `PLAN.md` with an initial approach, steps, and a `Repo Context` section.

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
