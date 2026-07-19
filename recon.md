# Lessons from Recon

This document records ideas evaluated from
[gavraz/recon](https://github.com/gavraz/recon), which is a tmux-native Claude
Code session dashboard. The comparison was refreshed against Recon's `main`
branch on 2026-07-13.

Orc and Recon overlap in session management, but their centers of gravity are
different:

- Recon is primarily a live provider-session browser.
- Orc is a durable, ticket-oriented workflow orchestrator.
- `STATE.yaml` remains authoritative in Orc. Provider and tmux data are optional
  live overlays and must never silently replace durable workflow state.

## Already brought into Orc

The first Recon-inspired implementation added:

- Exact tmux pane identity in `runtime.tmux.pane`.
- Orc identity metadata on tmux windows and panes.
- Safe pane validation and stale-pane recovery instead of active-pane guessing.
- Multi-pane live regression coverage.
- Attention-aware `orc watch`, urgency sorting, `i` navigation, and `orc focus`.
- `orc sessions` inventory for managed, stopped, orphaned, and optional
  unmanaged sessions.
- Optional Claude and Codex telemetry: provider session ID, model, effort,
  state, context usage, CWD, PID, and last activity.
- Exact provider correlation by explicit session ID and pane PID before CWD,
  including original/current identity merging across provider resume.
- `@orc_provider_session`, `@orc_provider_engine`, and `ORC_RESUMED_FROM`
  markers for resumable managed sessions.
- Provider-as-pane-process launches so tmux exposes an exact provider PID.
- Bounded head/tail transcript discovery with process-local incremental cursors
  and shared refresh budgets.
- An additive `live` overlay in `orc status <ticket> --json`.
- Guarded provider resume with discovery, active-process protection, CWD
  validation, exact argv, and dry-run support.
- Crash-safe, confirmed `orc sessions park` and `orc sessions unpark` flows.
- Repository and branch grouping from durable managed state plus cached Git
  metadata for orphaned and unmanaged sessions.
- Configurable live context-pressure warnings in `orc watch` and `orc dashboard`.

See [docs/sessions.md](docs/sessions.md) and [docs/watch.md](docs/watch.md) for
the implemented behavior.

## Completed recommendations

### 1. Exact provider-session correlation — implemented

Completed on 2026-07-13.

Orc now uses explicit provider identity and exact pane PID before considering an
engine plus CWD fallback. Multiple provider sessions can safely share a
directory without a newer unrelated session taking over a managed pane's live
overlay.

Recon preserves the original Claude session ID across resumes with a tmux
environment marker and falls back to process inspection when needed:

- [Recon session discovery](https://github.com/gavraz/recon/blob/main/src/session.rs)
- [Recon tmux resume implementation](https://github.com/gavraz/recon/blob/main/src/tmux.rs)

Implemented behavior:

- Orc stamps `@orc_provider_session` and `@orc_provider_engine` on resumed
  managed targets.
- Unpark sets `ORC_RESUMED_FROM` in both tmux and the provider process.
- Provider ID and PID matches outrank CWD; CWD is used only when no explicit
  provider identity exists.
- Launch scripts remove themselves and `exec` the provider, making it the pane
  process leader.
- When a resumed provider reports a different current process-session ID, Orc
  keeps the original resumable ID, exposes the current value as
  `observed_session_id`, and merges live state with transcript metadata.
- Missing or ambiguous explicit identity omits the overlay instead of guessing.

Provider versions that change transcript identity without updating any exact
process or session signal, including some Claude `/clear` behavior, remain
intentionally conservative: Orc will not rebind from CWD alone.

### 2. Incremental and tail-based telemetry parsing — implemented

Completed on 2026-07-13.

Recon keeps incremental state for active files and reads only a short tail for
resume history:

- [Recon live session parser](https://github.com/gavraz/recon/blob/main/src/session.rs)
- [Recon resume history scanner](https://github.com/gavraz/recon/blob/main/src/history.rs)

Implemented behavior:

- Cache by path, inode, size, and modification time.
- Continue from the previous byte offset for a growing live transcript.
- Invalidate safely when a file is truncated, replaced, or becomes malformed.
- Read a bounded head plus recent tail for the first historical summary.
- Share a 32 MiB and 250 ms parsing budget across each combined refresh.
- Retry incomplete records after append without retaining their bodies.
- Preserve the current rule that prompt and response bodies are neither retained
  nor printed.

The cache is process-local and metadata-only. New CLI invocations begin with a
bounded head/tail read; long-running consumers reuse exact cursors.

### 3. Search and an interactive resume picker — implemented

Completed on 2026-07-16.

Recon provides `/` filtering in its dashboard and an interactive resume picker:

- [Recon dashboard behavior](https://github.com/gavraz/recon/blob/main/src/app.rs)
- [Recon resume picker](https://github.com/gavraz/recon/blob/main/src/history.rs)

Implemented behavior:

- One shared case-insensitive, multi-term matcher drives `/` filtering in
  `orc watch` and `orc dashboard`.
- Existing durable metadata covers ticket, slug, workflow, stage, worker,
  repository, branch, status, engine, and attention state. Arbitrary labels
  remain deferred until Orc has a durable label schema.
- `orc sessions resume` without an ID opens a searchable picker showing
  provider, model, context, branch, CWD, and relative last activity.
- `orc sessions resume <provider-session-id>` remains the deterministic
  scripting interface.

Filtering should only affect presentation. It must not change focus priority or
workflow state.

### 4. Git-aware grouping — implemented

Completed on 2026-07-16.

Recon uses `git rev-parse --git-common-dir` to group worktrees under a canonical
repository and caches branch information:

- [Recon Git metadata](https://github.com/gavraz/recon/blob/main/src/session.rs)

Implemented behavior:

- Managed sessions project all repository, branch, and relative worktree data
  directly from `STATE.yaml.repos` without invoking Git.
- Orphaned and unmanaged sessions resolve canonical identity through
  `git rev-parse --git-common-dir` only when durable data is absent.
- Successful and failed Git lookups share a five-second process-local cache.
- Session inventory and resume search expose repository, worktree, and branch.
- Rows group by kind, repository, and branch without replacing ticket or stage
  as the primary workflow identity.

Example:

```text
los-app-los-django
  FLYWL-123  feature/flywl-123  develop  working
  FLYWL-456  feature/flywl-456  review   input
```

### 5. Context-pressure warnings — implemented

Completed on 2026-07-16.

Recon uses a colored context bar in its visual dashboard. Orc already exposes
context usage in session telemetry, so it can add a lighter version:

- Display context ratio in `orc watch` and `orc dashboard`.
- Use configurable green, yellow, and red thresholds.
- Treat missing or provider-unknown context limits as unavailable, not zero.
- Keep context pressure as a live warning. It must never become durable workflow
  status or automatically terminate or advance work.

Implemented behavior:

- Exact managed-session telemetry feeds both Live and Workspace without replacing
  `STATE.yaml` as the durable source of truth.
- The shared classifier defaults to green at 0%, yellow at 70%, and red at 90%;
  all three boundaries are configurable and validated in `orc.yaml`.
- Matched telemetry with no provider context limit renders `n/a`; sessions with
  no live telemetry render a dash.
- The overlay affects only rendering. It cannot reorder work, mutate state,
  advance a stage, or terminate a session.

### 7. Tmux popup workflow — implemented

Completed on 2026-07-16.

Recon documents popup bindings for its dashboard, new-session form, resume
picker, and next-attention command:

- [Recon tmux configuration](https://github.com/gavraz/recon/blob/main/README.md#tmux-config)

With Orc's resume picker in place, the optional bindings cover:

- `orc watch`
- `orc sessions --all`
- `orc sessions resume`
- `orc focus`

These remain user-owned tmux configuration rather than installation-time
mutation.

Implemented behavior:

- `docs/tmux.conf` provides syntax-checked bindings for watch, session inventory,
  interactive resume, and attention-driven focus.
- `docs/tmux.md` explains absolute and per-session workspace selection, popup
  behavior, a long-running resume pane alternative, and manual verification.
- Orc does not install, source, or mutate tmux configuration.

## Remaining recommendations

### 6. Durable labels and filters

Priority: optional. Add only when real workflows need it.

Recon stores repeatable `key:value` tags in the tmux environment and filters JSON
output by them. Orc already has ticket, workflow, stage, worker, repository, and
status, so many tags would duplicate existing structure.

If arbitrary labels become useful:

- Store them durably in `STATE.yaml`, for example `priority: high`,
  `team: frontend`, or `env: staging`.
- Validate keys and values.
- Add repeatable `--label key=value` filters to `orc status` and `orc sessions`.
- Mirror labels into tmux metadata only for live reverse lookup.

Do not make tmux environment variables the sole copy of Orc labels.

## Ideas not to copy directly

### Pane-text status scraping

Recon derives Claude state partly by scraping status-bar text from the pane.
That is useful for a provider-specific session browser but is brittle across
provider releases, themes, terminal widths, and localized UI text.

Orc should continue to prefer:

1. Durable `STATE.yaml` workflow state.
2. Explicit attention markers and provider metadata.
3. Pane text only as an optional diagnostic fallback, never as authoritative
   workflow state.

### One-key raw session killing

Killing a tmux session without updating durable state can leave an Orc ticket
marked active with no process. Orc should keep destructive lifecycle operations
explicit and confirmed. Parking, archive, and a possible future `sessions stop`
command should reconcile durable state deliberately.

### Generic session launch as the primary workflow

Recon's `launch` and `new` commands are natural for a general Claude session
manager. Orc already has `orc work`, `orc next`, and `orc jit`; a competing
generic launch path would bypass workflow policy and fragment ownership.

### Session-only tags

Ephemeral tmux tags are insufficient for an orchestrator whose state must
survive process and session loss. Any Orc labels should be durable first and
mirrored live second.

### Visual novelty before operational basics

Recon's Tamagotchi view is distinctive, but Orc first prioritized identity,
recovery, filtering, and context pressure. With those foundations complete,
`orc watch` now offers an optional little-orc pet view while retaining the rail
as its default and sharing the same operational controls.

## Recommended sequence

1. ~~Exact provider-session and pane identity.~~ Completed 2026-07-13.
2. ~~Incremental telemetry parsing with total refresh budgets.~~ Completed 2026-07-13.
3. ~~Search and interactive resume picker.~~ Completed 2026-07-16.
4. ~~Git-aware grouping and filters.~~ Completed 2026-07-16.
5. ~~Context-pressure presentation.~~ Completed 2026-07-16.
6. Durable labels remain optional and deferred until demanded by real use.
7. ~~Optional Tamagotchi view after the operational foundations.~~ Completed
   2026-07-17 with toggleable little-orc watch pets.
7. ~~Tmux popup documentation.~~ Completed 2026-07-16.

All practical v1 recommendations from Recon are complete. Durable labels are the
only remaining optional idea. Orc preserves its defining boundary: policy and
durable workflow state live in files; tmux and provider telemetry describe only
what is happening right now.
