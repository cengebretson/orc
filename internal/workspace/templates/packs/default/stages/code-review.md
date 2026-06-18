# Stage: code-review

> Before starting: read `ORC.md` for state update rules and error handling.

## Purpose

Review the implementation for correctness, spec compliance, and code quality before
a PR is opened. Automatically loops back to `develop` when changes are needed, up to
3 cycles. After 3 failed reviews a human is brought in.

## Cycle Counting

Before reviewing, count how many times `code-review` appears in `STATE.yaml` history
(entries where `stage: code-review`). This is the current cycle number.

- Cycle 1–2: if changes needed, route back to `develop` automatically
- Cycle 3: if still not approved, call `orc mark ... pause` — a human must resolve

## Steps

**Owner:** `default:zach` agent  
**Inputs:** feature branch code in worktree, `PLAN.md`, `SPEC.md`, `develop/HANDOFF.md`,  
`code-review/REVIEW.md` (from prior cycles)  
**Outputs:** `code-review/REVIEW.md` (overwrite with updated findings)

1. Read `SPEC.md` and `PLAN.md` to understand the intended design.
2. If `PLAN.md` has a `Repo Context` section, use it to check documented
   boundaries, impacted areas, and listed risks.
3. Read `develop/HANDOFF.md` for the implementation summary and known risks.
4. If a prior `code-review/REVIEW.md` exists, read it to understand what was previously flagged.
5. Review the code changes in the worktree (`git diff main` or the feature branch).
6. Check for: correctness, spec compliance, edge cases, security issues, test coverage.
7. If changed paths cross documented repo boundaries or touch undocumented areas,
   record that as a `[risk]` finding.
8. Write `code-review/REVIEW.md` with findings using tags: `[bug]` `[spec]` `[style]` `[risk]` `[minor]`
9. Set the verdict line: `**verdict: approved**`, `**verdict: needs-changes**`, or `**verdict: blocked**`.
   Valid values are defined in `code-review/REVIEW.md` — do not use custom verdicts.

## Exit Criteria

`code-review/REVIEW.md` is written with a clear verdict.

**If approved** — run:
```
orc mark <ticket> next --stage pr-open --worker <worker-id> --result "Code review passed"
```

**If needs-changes, cycle 1 or 2** — run:
```
orc mark <ticket> next --stage develop --worker <worker-id> --result "Review cycle <N>: changes needed — see code-review/REVIEW.md"
```

**If needs-changes, cycle 3** — run:
```
orc mark <ticket> pause "3 review cycles failed — see code-review/REVIEW.md for unresolved findings"
```

**If blocked** (design issue, spec conflict, missing requirement) — run:
```
orc mark <ticket> pause "Review blocked — <reason>. Human decision required before continuing."
```
