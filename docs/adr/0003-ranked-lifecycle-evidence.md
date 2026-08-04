# 0003. Lifecycle evidence is ranked, and only registration is authoritative

- **Status:** accepted
- **Date:** 2026-08-04

## Context

Orc shows whether an agent is idle, working, blocked, or done, and lets
automation wait on those states. Three sources can supply that, and they differ
in how much they actually know:

1. **Registration** — provider hooks, or a backend's native agent API. The agent
   says what it is doing.
2. **Terminal title** — both target engines write state into the OSC title. A
   braille spinner (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`, plus Claude's `· ✢ ✳ ✶ ✻ ✽`) means working;
   Codex writes `Action Required` when blocked; a title with no spinner reads as
   idle.
3. **Screen content** — pattern-matching what the pane is displaying.

Only the first is a report. The other two are guesses about a picture, and both
are readable without the user installing anything — which is the temptation,
because hooks only report for panes the user has installed them in, and an agent
started by hand is exactly the case a live rail is most useful for.

## Decision

All three sources are used, ranked, and labelled with their origin:

```
registration (hook | native)  >  title  >  screen
```

Every lifecycle and attention value carries a `source`. Inference never
overwrites newer registered metadata.

**Only registration is authoritative.** Inference is presentation-only. It may
not satisfy `orc ctl agent wait`, complete work, advance a stage, trigger a
notification of completion, or wake automatic parking. An agent Orc cannot
confirm publishes `unknown` — never a guessed settled state.

Title ranks above screen deliberately. It needs no content capture, no per-pane
polling, and no region model, and it covers the common cases for both engines
Orc targets. Screen scraping is a third tier, reached only when hooks are absent
*and* the title is uninformative: bounded regions, versioned per-engine rules
embedded in the binary, explicit priorities, debounced working-to-idle, and
optional workspace-local overrides with no fetched catalog.

Observation polls the whole server on a bounded interval in one call, not once
per pane and not once per frame.

## Consequences

An agent nobody registered still appears in Live views with a plausible state,
which is most of the value of a rail — while remaining unable to drive anything
that matters.

The cost is that a correct-looking state may not be actionable, and the
interface has to carry that distinction rather than hide it. `source` is not
diagnostic detail; it is the field that decides whether a state can be acted on.

Inference is engine-coupled: a title vocabulary or spinner glyph set that
changes upstream degrades detection to `unknown`. Degrading is the accepted
failure mode. Rules are versioned so they can be corrected without a release.

Display labels must strip the leading activity glyph, or a row flickers on every
spinner frame — the raw and display forms both have to be kept.
