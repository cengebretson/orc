# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.27.0] - 2026-08-18

### Added

- A live contract test against the real `tmux-attention` CLI. Every other test
  asserts option names Orc hardcodes, which is how the two projects silently
  disagreed: Orc read `@agent_attention_since` while the plugin wrote
  `@agent_pane_attention_updated_at`, and both suites stayed green because
  neither ever saw the other's output. The new test drives the plugin and reads
  the result back through `ListPanesDetailed`, so a rename on either side fails.
  Verified by renaming the option in a copy of the plugin: the contract test
  fails while the unit tests still pass. Skips when the plugin is absent, since
  it is an optional dependency ([ADR 0005](docs/adr/0005-orc-and-tmux-attention-are-layered.md));
  `ORC_TMUX_ATTENTION_CLI` points it at a specific copy.
- The MIT license and a comparison guide that defines Orc as the durable
  workflow layer rather than a terminal, multiplexer, tracker, or IDE.
- `orc demo`, a zero-setup entry point for the synthetic Live dashboard, with
  the dashboard preview promoted near the README quick start.
- `orc events` with optional follow and JSONL output for deterministic feature,
  attention, session, and stage changes from immutable workspace snapshots.
- Feature-detail actions to open or copy canonical ticket and PR links from
  `TICKET.md` and show pull-request checks through the GitHub CLI.

### Changed

- Reframed the README around durable plans, decisions, state, and handoffs,
  with multiplexers documented as execution backends rather than the product.

## [0.26.0] - 2026-08-16

### Added

- Read `tmux-attention`'s `@agent_pane_context_active` as a `context`
  observation source, ranked above title and screen inference. An agent hook
  reported the running turn, so it beats guessing from a picture — and it lets
  Orc show `working` for a managed pane whose agent reports through the plugin
  rather than through Orc's own hooks, which it previously could not see at all.
  It remains an observation: Orc cannot verify who wrote it, so it never
  satisfies a registered-source check. Only an explicit `1` counts; empty means
  the plugin is absent and a cleared value is a finished turn.
- [ADR 0005](docs/adr/0005-orc-and-tmux-attention-are-layered.md) recording why
  Orc and `tmux-attention` stay layered and neither absorbs the other.

## [0.25.0] - 2026-08-16

### Changed

- Only named sources may drive actions. The gates deciding whether a lifecycle
  or attention value could stand as a pane's live state, satisfy an `orc ctl`
  attention wait, or wake parked work excluded the sources known to be
  untrustworthy, which made "authoritative" the default for anything
  unrecognized. `@agent_attention` is a shared namespace and the
  `tmux-attention` CLI takes a free-text `--source`, so markers written by
  Claude and Codex hooks — or by any script — qualified.
  Authority is now a positive list of `hook` and `native`, the two channels Orc
  can verify. See [ADR 0004](docs/adr/0004-authoritative-sources-are-named.md).
- As a result, `tmux-attention` markers no longer wake parked work or satisfy an
  `orc ctl` attention wait. Orc still reads and displays them; it no longer acts
  on a report it cannot confirm. Attention carrying no source at all is likewise
  display-only — no writer emits that today, but it was previously actionable.
  Stage advancement is unaffected: that happens when an agent or a human runs
  `orc mark <ticket> next`, and no signal gated here can move a pipeline.

### Added

- `orc jit --consult` regression coverage, and `jitConsult` is now saved and
  restored by the test globals helper so it cannot leak between tests.

## [0.24.0] - 2026-08-16

### Added

- `orc jit --consult` for advisory runs. A ticket allows one JIT task at a time
  because a JIT task owns `runtime.jit` until closed, which meant an agent
  already inside one could not consult the advisor worker — exactly when advice
  is most useful. A consult run neither claims the slot nor is blocked by one,
  writes no runtime state, and drops the closing `orc mark <ticket> jit` step
  since it opens no task.

### Changed

- `orc doctor` no longer recommends `orc hooks install` when something else
  already owns the events Orc would claim. Orc uses SessionStart,
  UserPromptSubmit, PermissionRequest, PostToolUse and Stop (plus Notification
  and StopFailure on Claude); for a setup with its own hook dispatcher that is
  nearly every event it uses, and installing over it leaves two systems writing
  agent state on one event with last writer winning. Doctor now reports the
  conflicting hooks and says to review instead.

### Fixed

- Parse tmux pane rows by field name rather than position. The format string and
  the parser were coupled by index with nothing enforcing it, so inserting a
  field mid-list shifted every later one and a pane's stage surfaced in its
  worker column — wrong data with no crash and no failing test. Both are now
  generated from one ordered list.
- Stop `TestDetectVersionUsesFirstNonEmptyLine` failing on machine load. The
  probe's 2s bound is wall-clock, and spawning a freshly written script under a
  parallel `go test ./...` can exceed it; the bound is now overridable so the
  test does not race it. The production default is unchanged.

## [0.23.0] - 2026-08-16

### Added

- `default:vera`, an advisor worker in the default pack, for consulting a second
  agent without delegating the work. Runs through the existing `orc jit` path —
  `orc jit <ticket> --worker default:vera "<question>"` — so the exchange lands
  in `features/<slug>/jit/<timestamp>/` and outlives the session that asked.
  Distinct from `default:ada`: Ada plans and her plan becomes the work, whereas
  Vera answers a question and hands it back to a caller who keeps the work. Its
  "advise, do not act" charter is carried in `args.append-system-prompt` rather
  than the worker's body, because `orc jit` builds its own prompt and never
  renders a worker's Prompt Template — a constraint written only in the markdown
  would not reach the model.

### Fixed

- Publish attention markers under `tmux-attention`'s own pane schema
  (`@agent_pane_attention{,_updated_at,_source}`) alongside the existing
  `@agent_attention` trio. Orc wrote only the latter, which renders the tab
  glyph — that format resolves through pane → window → session — but left every
  orc-set marker invisible to the plugin's CLI and absent from
  `tmux-fzf-jump`'s attention picker, both of which read the pane-scoped
  option. Written as plain tmux options rather than by shelling out, so orc's
  lifecycle tracking still works without the optional plugin installed.
- Read a marker's age from `@agent_pane_attention_updated_at`, falling back to
  `@agent_attention_since`. The plugin records the former and leaves the latter
  empty, so a marker set by a Claude or Codex hook was read with its state but
  an age of zero, flattening every age-derived display in `orc watch`.

## [0.22.1] - 2026-08-05

### Changed

- Moved lifecycle-hook JSON parsing, subagent filtering, provider-session
  extraction, and deterministic event-ID generation into the Orc binary. The
  installed fail-open Bash hook now forwards provider events over stdin and no
  longer requires Python.

## [0.22.0] - 2026-08-04

### Added

- Added `orc run "<instruction>"` for standalone local work. It
  allocates a durable workspace-local `LOCAL-N` ID, derives a filesystem-safe
  slug from the instruction, creates a normal feature using the bundled
  single-stage `default:adhoc` workflow, and launches through the same runtime,
  rail, lifecycle, control, completion, and archive paths as tracked work.
  `--slug` overrides the derived name; `--worker` and `--repo` skip interactive
  selection (with repository inference from the current directory); and
  `--tmux`, `--attach`, and `settings.auto_tmux` control multiplexer launch.
  Non-interactive use reports the available explicit flag values instead of
  waiting for input. Older workspaces receive the missing local workflow and
  stage guide additively on first use.
- Added an operator-phrase table with underlying commands to the generated
  `ORC.md` agent contract for concise status, validation, handoff, pause, resume,
  completion, and help requests. These phrases map to the current ticket's
  durable protocol and are explicitly distinguished from literal shell
  commands.

### Changed

- Tightened generated Markdown contracts: `SETUP.md` now identifies `ORC.md` as
  the durable protocol owner, the final QA stage gives an explicit `orc mark
  <ticket> done` transition instead of implying archival, and operator shorthand
  accepts authoritative end commands from either the launch prompt or stage
  instructions.
- Made `RULES.md` authoritative for final external-automation decisions in the
  PR-open and QA stages: configured workspace exceptions may permit full
  automation, while unapproved pushes, PR creation, CI actions, or ticket
  updates pause with the exact proposed action. The shared `AGENTS.md`, `ORC.md`,
  and `RULES.md` templates now state this precedence explicitly so stage,
  worker, prompt, and operator instructions cannot implicitly grant permission.

## [0.21.0] - 2026-08-04

### Changed

- Made `AGENTS.md` the concise, tool-neutral source of repository agent
  guidance and changed `CLAUDE.md` to import it, keeping Codex and Claude on the
  same behavioral contract without duplicating instructions.
- Streamlined the documentation set: the README now uses a compact command map,
  `docs/workflows.md` is the canonical complete `orc.yaml` reference, and the
  reference and watch documents avoid duplicated schema and spec-like prose.
- Added focused links for version, watch, rail, workflow configuration, and
  backend behavior so detailed guidance remains discoverable after the cleanup.

### Fixed

- Updated the isolated agent-hook verification script to use the current
  `orc hooks install` command and corrected the watch-to-tmux documentation
  anchor.

## [0.20.0] - 2026-08-04

### Added

- Added durable labels. `orc label <ticket> key=value` sets them,
  `--remove <key>` deletes them, and a bare `orc label <ticket>` lists them.
  Orc assigns no meaning to any key and no transition reads them, so a label
  can never change what a stage does — only which features a view shows.
- `orc status --label` and `orc sessions --label` filter by `key` or
  `key=value`. Repeating the flag intersects, so `--label area=api --label
  priority=high` requires both. Key and value comparison is case-insensitive,
  matching how the interactive filters already behave.
- `orc watch`'s `/` filter searches labels too, so `/area=api` narrows the rail
  without a separate label syntax, and `orc status <ticket>` lists them.
- Added per-worker time attribution. `orc report --by-worker` shows total,
  average, and median active time plus run counts per worker, ordered by total
  so the most expensive leads; `orc report <ticket> --by-worker` breaks one
  ticket down. It derives from the same durable history the stage report uses,
  so it needs no new data collection.
- Attribution reports time, not tokens or currency. Cumulative token usage is
  not exposed alike by both providers today, and converting it would need a
  per-model price table Orc would have to keep current — a number that looks
  precise and is quietly wrong is worse than one that is honestly coarse.
- Added a `terminal` theme that derives dashboard and watch colors from the
  terminal's own palette. Accent roles become ANSI slot indices the terminal
  maps, and body text is left unset so its default foreground shows through —
  the only choice that stays readable on light and dark backgrounds alike.
  Select it with `settings.theme: terminal` in `orc.yaml`.
- `orc doctor` now validates `settings.theme` and lists the available names. An
  unknown theme falls back to the default silently, so without the check a typo
  looked like the setting simply did nothing.

## [0.19.0] - 2026-08-04

### Added

- Added structured human responses. `orc mark <ticket> pause` now takes
  `--confirm`, `--choice <key>=<label>`, or `--text` to declare the shape of
  answer it needs, and `orc answer <ticket> [value]` answers it — printing the
  question and its accepted answers when no value is given.
- The answer is recorded in `STATE.yaml` under `runtime.question` before any
  attempt to reach the live agent, and the next launch prompt leads with it.
  Previously the only reply channel was typing into the agent's pane, so an
  answer was lost whenever that process exited, was replaced, or the work was
  parked — and the next agent asked the same question again.
- Orc validates that an answer is one the question offered and refuses anything
  else, leaving the question pending. It never interprets what an option means:
  that stays with the agent that asked.
- `orc watch` answers questions in place. `s` on a ticket with a pending
  question opens the control its kind calls for — `y`/`n` for confirm, a
  pick-list for choice, a text box for text — and falls back to the existing
  free-text prompt when nothing was asked. Answering needs no live agent, so a
  ticket whose agent has already exited can still be answered.

## [0.18.0] - 2026-08-04

**Breaking:** `orc doctor --install-agent-hooks` is now `orc hooks install`
(`--dry-run` moves with it). Hook installation writes into your Codex and
Claude configuration rather than the workspace, so it is its own command;
`orc doctor` still reports whether the hooks are installed.

### Added

- Added two guards for release notes. `scripts/release-contract` now rejects a
  `.goreleaser.yaml` that sets `changelog.disable`, catching the cause on every
  CI run rather than at tag time; the release workflow's new
  `scripts/verify-release-published` step compares the published release body
  against the notes GoReleaser was given, covering the failure modes a static
  check cannot anticipate.

### Changed

- Hook installation moved from `orc doctor --install-agent-hooks` to
  `orc hooks install [--dry-run]`. It writes into Codex and Claude
  configuration rather than the workspace, so it was never a repair of anything
  `doctor` diagnoses — it rejected doctor's ticket argument, `--fix`, and
  `--system`, and needed three guard clauses to stay separated from them.
  `orc doctor` still reports whether the hooks are installed and now names the
  new command; `--fix` stays on `doctor`, where repairing a stale workspace lock
  is exactly what a doctor should do.

### Fixed

- GitHub Releases now carry their changelog section as the release body.
  `.goreleaser.yaml` set `changelog.disable`, which skips the changelog pipe —
  and that pipe is what reads `--release-notes`. Every release up to and
  including `v0.17.0` published with an empty body as a result; `v0.17.0`'s
  notes were restored by hand. `make release-check` could not have caught this,
  because snapshot mode skips the same pipe and never renders a release body.

## [0.17.0] - 2026-08-03

**Breaking:** five `orc watch` flags are gone. Replace `--tmux-toggle`,
`--tmux-layout`, and `--tmux-size` with `orc rail toggle [--layout] [--size]`;
`--view` and `--pet-layout` have no replacement. Check tmux keybindings that
invoke `orc watch` — an unknown flag now fails the command.

### Added

- Hardened `make release-check` with a shared `VERSION`/tag/changelog contract,
  GoReleaser `v2.17.1` pin, non-publishing four-platform snapshot build,
  artifact-name and checksum verification, and the same gate in CI. The tag
  workflow now consumes the shared contract and exact GoReleaser pin.
- Added `CONTEXT.md` as the concise developer glossary for Orc's domain terms,
  durable/live state boundaries, and stable architectural vocabulary.
- Added conservative tmux fallback detection through embedded versioned Codex
  and Claude manifests, strict workspace-local overrides, title-over-screen
  precedence, bounded capture regions, working-to-idle debounce, and explicit
  unknown state. Inference is presentation-only and cannot satisfy control
  waits, notify completion, advance workflow, or wake automatic parking.
- Added runtime reconciliation across exact live, resumable, replaced,
  orphaned, and contradictory agent evidence. Parking now snapshots proven
  identity and authoritative lifecycle; restoration creates a fresh instance,
  clears inherited metadata, and waits for a provider hook before committing
  the restored runtime target.
- Added a managed, mouse-resizable tmux attention rail with ownership-safe
  open, close, toggle, collapse, and expand commands; configurable age/stuck
  rendering; conservative seen acknowledgement; and deduplicated authoritative
  block/completion notifications.
- Added safe exact-instance prompting for tmux through private stdin-loaded
  buffers, bracketed paste, encoded submission, and automatic cleanup. Waiting
  prompts now require an authoritative hook transition and distinguish stalls,
  timeouts, replacement, and agent exit; `orc watch` adds an explicit compose,
  review, and `y`-confirm prompt action.
- Added exact-instance tmux lifecycle reads and context-cancelable waits to
  `orc ctl agent state`, `wait`, and `watch`. Stable replacement, offline,
  invalid-state, cancellation, and timeout codes now match the structured
  control contract, and new launches reset inherited pane state to `unknown`.
- Added an idempotent Codex and Claude lifecycle-hook installer through
  `orc doctor --install-agent-hooks`, with dry-run planning, explicit Codex
  trust guidance, foreign-hook preservation, atomic writes, and doctor health
  reporting for idle, working, and blocked states.
- Added the first Super Orc identity foundation: durable agent and live-instance
  state, pre-launch tmux identity stamping and provider environment, exact-
  instance validation, and a deduplicated hook event command that publishes
  sequenced lifecycle and attention metadata without trusting an arbitrary pane.
- Completed the structured `orc ctl` surface with aggregate session status,
  transition-only agent lifecycle JSONL watching, and exact-target terminal
  capture for Herdr and tmux.
- Added opt-in reversible parking for Live views. Configured statuses collapse
  into an expandable `Parked (n)` group without stopping any session, while
  status, attention, or stage changes wake and visibly flag the ticket.

### Changed

- Documented the global `--workspace` and `--mux` flags and the hook-invoked
  `orc agent-event` command in `README.md`.
- Removed unreferenced internal helpers left behind by the `mux.Backend`
  refactor (`sessionlist.ManagedTelemetry`, `watch.Focus`,
  `workspacesnapshot.LoadItems`, `tmux.SendCommand`, `runner.ResolveWorkflow`,
  `ticket.ResolveWithArchive`, and `health.Print`), along with the remaining
  nil-backend wrappers and superseded renderers that only tests still reached.
  Also removed `tmux.ValidateAgentTarget`, a third copy of the agent-identity
  check `readAgentState` already performs on every control call, and the
  superseded whole-file telemetry readers (`readClaudeJSONL`,
  `readCodexJSONL`, `scanJSONL`) left behind when discovery moved to
  incremental cursors. Their coverage moved onto the live code paths. No
  user-facing behavior changed.
- Context-pressure colouring and the trend sparkline now come from one
  implementation in `internal/ui`, shared by the Live rail and the Workspace
  tabs, instead of five copies of the level switch and two of the sparkline.
- Consolidated duplicated internal helpers so the copies can no longer drift
  apart. Shell quoting moved to `internal/shellquote`, which now distinguishes
  always-quote (`Quote`, required where hook commands are matched against
  config written by earlier versions) from quote-only-if-needed (`Word`).
  Path comparison, relative-time and empty-value rendering, and the loop-count
  stage label each have a single implementation, and the stricter of each
  previously divergent pair won: an empty path is no longer "the same path" as
  another empty path, a zero timestamp renders as `-` rather than `now`, and
  whitespace-only values render as `-`.

### Removed

- Removed the little-orc pet view. `orc watch --view`, `--pet-layout`, and the
  `v` and `l` keys are gone; the rail and its wide table are now the only Live
  presentations.
- Removed `orc watch --tmux-toggle`, `--tmux-layout`, and `--tmux-size`. They
  were aliases that already delegated to the managed rail; use
  `orc rail toggle [--layout <right|bottom>] [--size <n>]` instead. Note the
  default size changes from 32 to the rail's 64 columns.

### Fixed

- `orc mark --help` and `orc help-all` now list the `jit` verb, which the
  command has always accepted but never advertised.
- Untracked the 18 MB `cmd/orc/orc` build artifact, committed by accident in
  `69b3817`, and added it to `.gitignore` so it cannot come back. The blob
  itself stays in history by choice: it is an ancestor of the `v0.16.0` and
  `v0.16.1` tags, so removing it would rewrite both published releases for
  8.3 MiB that costs nothing to keep.
- The live tmux test now skips, rather than fails, where tmux is installed but
  cannot start a server or spawn a shell in a temp directory — the case in
  sandboxed and containerized environments.

## [0.16.1] - 2026-08-02

### Fixed

- Updated the GoReleaser build-date template for GoReleaser 2.17 so release
  archives can be built after `.Date` changed from a time value to an RFC 3339
  string.

## [0.16.0] - 2026-08-02

### Added

- `orc ctl agent state`, `agent prompt`, and `agent wait` provide structured
  control of an exact recorded Herdr agent. State reads return Herdr's current
  recognized lifecycle without changing focus; waiting delegates to Herdr and
  preserves distinct timeout and `agent_prompt_stalled` errors instead of
  inferring completion from terminal text.
- Tickets running through Herdr now publish native session notifications when
  they block or complete, using distinct request and completion sounds without
  replacing the existing user-configured notification command.
- Herdr launches can optionally build an Orc-owned task cell beside each stage
  agent: a configured test command runs in a right-side pane, `orc watch` can
  run below it, and metadata-backed ownership makes repeated launches reuse the
  utility panes without adopting user-created panes.
- Herdr launches can now create a ticket's Git worktree through
  `herdr worktree create` or reopen its exact recorded checkout through
  `herdr worktree open`. Orc records the repository, branch, and checkout in
  `STATE.yaml` before launching the agent and starts the worker in that
  worktree; repository-specific `worktree_setup` commands retain precedence.
- Workspace-configurable transition notifications can run a short command after
  tickets block or complete a stage. Commands receive template placeholders and
  `ORC_*` environment variables; failures warn without rolling back durable
  state.
- A native Herdr multiplexer backend is available with `--mux herdr`. Orc can
  create ticket workspaces and stage tabs, launch Claude or Codex through
  Herdr's agent API, persist exact workspace/tab/pane IDs, inventory lifecycle
  state, attach to the recorded pane, and publish Orc identity tokens for
  Herdr's sidebar.
- Live runtime identity is now backend-neutral in `STATE.yaml` under
  `runtime.mux`. Existing `runtime.tmux` state remains readable without an
  eager migration, while new launches record the backend and opaque target IDs.
- Dashboard tabs are clickable — clicking a tab label in the header switches
  to it, matching the existing `1`-`5` and arrow-key navigation.
- Each worker now gets a stable accent color, assigned so no two workers
  collide, shown consistently everywhere a worker appears — the Features
  table, Workers tab, ticket detail, and the Workflows stage table.
- The Health badge briefly pulses when its issue count changes on refresh
  (not on the very first load), so a newly-appeared or newly-resolved issue
  catches the eye instead of silently updating the number.
- The empty Features view now shows a rotating quote alongside the "no
  features" message instead of flat text, and rotates to a new one every 8s
  while idle and still empty.
- The Features table's Context column now shows a small sparkline of each
  live feature's recent context-pressure trend alongside the percentage,
  built from samples collected on the 2s live refresh.
- Features needing attention (blocked, input, review) now get a colored
  marker bar in the Features table, matching their status color, instead of
  relying on scanning the Status column text.
- The operational banner's "NEEDS YOU" count now breathes gently while it's
  above zero — a slow, continuous dim/brighten cycle rather than a one-shot
  flash — so an ongoing attention need reads as ambient rather than urgent
  noise.
- Repositories now get the same stable-accent-color treatment as workers,
  shown in the Repositories view's list and inspector cards.

### Changed

- The operational banner's staleness indicator is now a small fill bar and
  countdown toward the next full refresh (`↺ ▓▓▓░░░ 23s`) instead of a
  static "Ns ago" timestamp, while still turning yellow/red on genuine
  staleness the same way it always has.
- The Workers detail view is more consistent and complete: the worker card
  is now merged into the pinned Workers panel, which includes the selected
  worker's configuration, one-line role summary, and active-feature count.
  Active Features always spans the full width (previously half-width side by
  side on wide terminals) with new Status and Context columns, and the
  markdown body below is now wrapped in a panel like the rest of the dashboard
  instead of flowing unboxed.

- The ticket detail page and any drilled-into file viewer (a stage file, a
  worker opened from its section list, a ticket's linked document) now pin
  the shared operational banner above their scrollable content too, so it
  stays visible while scrolling instead of scrolling away with the rest of
  the page — consistent with every other Workspace view.

- Workflows now opens directly into its route chain and stage table, filling
  the terminal height with the shared operational banner pinned above it,
  matching Health, Workers, and Repositories instead of landing on a sparse
  collapsed summary. The selected workflow and stage cursor are preserved
  across tab switches.

- Promoted the merged Live/Features view, Workflows, Workers, Repositories, and
  Health into a unified dashboard tab bar with cyclic and direct-key navigation,
  dedicated Workspace section views, preserved state, an adaptive compact
  header, and a shared operational banner. Features now folds in Live attention
  states and uses a lightweight two-second session refresh without rerunning
  slower Health checks;
  narrow dashboards switch to the Live rail and restore the selected tab when
  widened, Health tabs badge non-OK checks in yellow, and `orc watch` remains the
  dedicated Live rail. Health now opens its grouped checks directly beneath a
  pinned summary, fills the available terminal height, and removes the redundant
  report-title drill-in. Config validation is consolidated into the `orc.yaml`
  panel, state-lock checks are consolidated into Workspace, and redundant
  healthy config rows are omitted. Repositories now opens directly into a
  pinned routing summary with responsive two-column repository and route cards,
  removing its inspector drill-in. Repository paths are resolved for display,
  use `~` home abbreviation, and identify the workspace root without exposing
  raw relative traversal. A hidden `0` destination selects the Orc
  title and brings back the original portrait and configured quotes,
  automatically runs the classic rainbow animation, rotates to a new quote on
  each visit, and shows
  version, build, revision, and workspace metadata.

## [0.15.1] - 2026-07-19

### Added

- Added `make release-check`, a deterministic fresh-workspace smoke test that
  validates agent setup output, repository configuration, first-ticket doctor,
  lifecycle transitions, archive behavior, and both initialization variants;
  CI now runs it on every push and pull request.

### Fixed

- Attention markers are now read per pane and rolled up per window, so a window
  running more than one agent — a `orc jit --tmux` task sent into a stage's
  session, or a split you made yourself — no longer reports whichever agent
  wrote its marker last. The most urgent state wins (`blocked`, then `input`,
  then `review`, then `done`), so a blocked agent can't be hidden behind a
  finished one, and when two agents share a state the elapsed time tracks
  whichever has been waiting longest. Markers set only on the window keep
  working unchanged.
- Unrecognized attention markers are treated as no signal instead of being
  passed through to the display.
- Included the generated `.gitignore` in the shared atomic workspace mutation
  plan so initialization counts it, reports write failures, and rolls it back
  consistently with every other scaffold file.

## [0.15.0] - 2026-07-19

### Changed

- Simplified the terminal UI around shared work-item projection, view state,
  rendering primitives, and consolidated regression coverage.
- Made workspace initialization and workflow-pack installation use the same
  preflighted, atomic mutation plan, including rollback of created and updated
  files when a write fails.
- Split state persistence, tmux integration, and CLI registration into focused
  modules, with tmux process execution isolated behind a testable boundary.
- Expanded direct coverage of state transitions, tmux targeting, stage files,
  parked-session cleanup, and provider-session selection.

### Fixed

- Stamped a new feature's configured first-stage worker into
  `STATE.yaml.next_action.worker`, so ticket-scoped `orc doctor` succeeds
  immediately after `orc work`.

## [0.14.0] - 2026-07-19

### Changed

- Renamed `settings.tui_refresh` to `settings.workspace_refresh` to reflect
  that it controls only Workspace polling; Live polling remains governed by
  the `orc watch --interval` option.
- Unified Live, Workspace, and dashboard theming behind shared terminal-cell
  layout primitives, renamed the Workspace implementation package for clarity,
  and stopped hidden dashboard sections from polling or animating.
- Consolidated dashboard discovery into immutable workspace snapshots, made
  structured inspectors refresh in place, and surfaced invalid `orc.yaml`
  configuration as Health state instead of allowing a nil-config panic.
- Hardened headers, tables, boxes, wrapping, and repository inspectors for ANSI,
  Unicode, and narrow terminal widths.
- Added consistent inner padding plus durable active and paused feature counts
  and ticket IDs to repository inspector cards.
- Polished the default `orc watch` rail with a live and attention summary,
  stronger selection and action hierarchy, repository and stage context, a
  compact context-pressure meter, durable activity age, and context-aware
  navigation available through the `?` help overlay instead of persistent
  footer legends.
- Added a read-only `orc watch --demo` presentation, workflow progress routes,
  in-memory context sparklines, live pulses, transition and completion
  treatments, a key-help overlay, and a richer expanded details page covering
  state, repository, runtime, next action, sticky scroll progress, workflow
  position and timing, and connected history.
- Added `orc dashboard` as the unified Live and Workspace application with
  state-preserving `[`/`]` section navigation. Wide watch uses the same shell,
  narrow watch retains its compact presentation, and the former `orc tui`
  command has been removed.
- Removed redundant watch title banners and replaced the plain live/attention
  count with a compact Workspace-style overview card and live refresh age.
- Embedded Live detail section titles into their respective panel borders and
  removed the redundant `NOW` / `NEEDS YOU` selected-card headings.
- Replaced the details `STATE` label with the feature name and ticket, and
  removed the redundant details-page header.
- Consolidated provider and live runtime metadata into the feature panel and
  removed the separate Runtime panel.
- Applied configured `orc.yaml` stage aliases throughout watch while retaining
  canonical stage IDs for runtime targeting and search.
- Made the wide watch table distribute all available terminal width across its
  Ticket, Stage, Worker, and Tmux columns instead of leaving capped dead space;
  the selected-work panel now spans and aligns with that table.
- Renamed watch's tmux-derived `LIVE` count to `RUNNING` and added the durable
  paused-feature count alongside the separate `NEEDS YOU` attention signal.
- Split the Health drill-in into a status summary and color-coded panels per
  health group, with wrapped diagnostics kept inside their owning section.
- Made the dashboard Repositories section collapsed by default for a tighter
  initial workspace overview.
- Added explicit line, page, half-page, and top/bottom scrolling to Health and
  other structured viewers, with live scroll percentage in the viewer title.
- Applied the same scrolling controls and progress indicator to Workflow
  details, leaving left/right navigation dedicated to stage selection.
- Renamed the dashboard Routes section to Repositories, added repo paths and
  optional rule mappings to its overview, and replaced raw `orc.yaml` drill-in
  with a structured repository map, individual repo cards, and explicit
  optional-routing flow cards.

## [0.13.0] - 2026-07-18

### Fixed

- Made `orc watch` truncate Unicode text by terminal cell width, preventing
  broken UTF-8 and misaligned rows for non-ASCII ticket and session metadata.
- Made `orc.yaml` reject unknown keys, missing or duplicate repository
  identities, duplicate repository paths, and negative TUI refresh intervals so
  agent-authored setup mistakes fail visibly instead of falling back silently.

### Changed

- Streamlined the generated `SETUP.md` into an agent-run discovery, confirmation,
  configuration, and verification workflow that asks users only for decisions
  the agent cannot safely discover.
- Tightened the generated `ORC.md` state contract by separating orc-owned
  lifecycle fields from agent-writable handoff context and removing duplicated,
  drift-prone command and artifact guidance.
- Moved repository routing into validated `orc.yaml` rules that map exact ticket
  labels or components to one or multiple repositories, while keeping workflow
  selection independent. `ORC.md` makes routing a precondition for any
  repository-dependent stage, including custom workflows without intake;
  `TOOLS.md` owns ticket retrieval and `RULES.md` contains only approval policy.
- Changed generated `orc.yaml` repository examples to use setup-discovered
  absolute paths instead of assuming repositories are workspace siblings;
  existing relative paths remain supported.
- Reduced generated `AGENTS.md` to a stable workspace entrypoint that delegates
  routing, lifecycle, permissions, and tools to their owning contracts instead
  of duplicating those instructions.

### Removed

- Removed generated `ROUTER.md`; its structured repository-routing role now
  belongs to `orc.yaml`, eliminating a second routing source of truth.

## [0.12.0] - 2026-07-17

### Changed

- Replaced the monochrome ASCII little-orc sprites in `orc watch --view pet`
  with true-color half-block pixel art (flared ears, tusks, a mohawk tuft),
  rendered two pixel rows per terminal line so every mood reads distinctly at
  a glance; per-agent identity color still rides the same deterministic hash
  as before. Celebrating throws its arms overhead on the animation tick, and
  needs-input shows a "?" above its head instead of relying on brow flecks
  alone.

### Removed

- Removed the three-row micro pet sprite size (`s` key, `--pet-size` flag)
  from `orc watch --view pet`. A detailed pixel-art creature doesn't hold up
  at that resolution; there is now a single sprite size.

## [0.11.0] - 2026-07-17

### Added

- A toggleable `orc watch` Tamagotchi view with animated little orcs, responsive
  pet cards with a focus-only outline, optional three-row micro sprites and
  vertical-column layout, deterministic identity, attention and
  context-pressure moods, and the same filtering, preview, attach, focus, and
  refresh controls as the default rail.

## [0.10.1] - 2026-07-17

### Fixed

- Session-resume picker tests no longer depend on a locally installed Codex
  binary, keeping the release suite portable to clean CI runners.

## [0.10.0] - 2026-07-17

### Added

- A release-readiness runbook covering v1 scope, automated gates, disposable
  workspace QA, live session recovery, and tag/artifact verification.
- Copyable, user-owned tmux popup and split-pane bindings for watch, session
  inventory, interactive resume, and attention-driven focus.
- Live provider context pressure in `orc watch` and `orc tui`, with configurable
  green/yellow/red thresholds and explicit unavailable rendering when a provider
  does not report its context limit.
- Repository and branch grouping for `orc sessions` and the resume picker,
  sourced from durable `STATE.yaml.repos` for managed work and a short-lived
  `git-common-dir` cache for orphaned or unmanaged sessions.
- Shared multi-field `/` filtering in `orc watch` and `orc tui`, plus an
  interactive `orc sessions resume` picker when no provider session ID is
  supplied; explicit IDs remain available for deterministic scripts.
- `orc sessions` inventories managed, orphaned, and optional unmanaged Claude
  and Codex sessions with bounded live telemetry, exact provider-session
  resume, and crash-safe park/unpark workflows.
- `orc watch` and `orc focus` provide an attention-aware live session rail and
  deterministic navigation to the next session needing human input.

### Changed

- Managed tmux launches now record and validate exact agent-pane and provider
  identity, recover conservatively from stale panes, and incrementally refresh
  transcript metadata without retaining prompt or response bodies.
- Tmux pane inventory now preserves sessions without attention metadata, and
  unpark retries reconcile exact already-restored panes while rejecting unrelated
  session-name collisions.
- Roadmap and watch documentation now distinguish the completed v1 surface from
  explicitly deferred post-v1 work.

## [0.9.0] - 2026-07-06

### Fixed

- Launched agents now abort if they cannot `cd` into the recorded worktree
  instead of silently starting in the wrong directory and running repo commands
  against the wrong tree.

## [0.8.0] - 2026-07-06

### Added

- `orc mark <ticket> next --force` lets a human advance past
  `artifact_policy: block` when required artifacts are not ready; the skipped
  artifacts are recorded in the stage result and STATE.yaml history.

### Changed

- Required-artifact readiness checks now flag stage artifacts that are still
  byte-identical to the feature template as `unchanged from template`, so an
  untouched scaffolded doc no longer counts as done. Core docs remain a
  presence-only check.
- With `artifact_policy: block`, `orc mark <ticket> next` now reports the
  `advance: manual` pause guidance (and loop-limit pause/fail outcomes) before
  the artifact readiness error, so agents are steered to the escape valve first.
- `orc next` now surfaces a worktree setup command that fails placeholder
  expansion in the launch prompt instead of silently omitting the setup step,
  and required-artifact reminders show the real feature folder path instead of
  `features/<slug>/`.

### Fixed

- `orc doctor` no longer warns "command not found in PATH" for `worktree_setup`
  commands that start with a shell builtin such as `cd` or `source`.
- Worktree reconciliation findings in `orc doctor <ticket>` and `orc validate`
  are now reported in stable repo-name order.

## [0.7.2] - 2026-07-06

### Added

- `orc artifacts <ticket>` reports required artifact readiness for the current
  stage, with `--all` for workflow-wide checks and `--json` for automation.
- `orc doctor` now warns when repo `worktree_setup` commands are missing or not
  executable, when custom setup lacks `agent_hints`, and when `worktrees/` is
  absent for worktree-driven repos.
- Generated `SETUP.md` now asks setup agents to inspect repo entrypoints when
  users are unsure, capture durable `agent_hints`, configure
  `required_artifacts`, and verify with `orc artifacts`.

## [0.7.1] - 2026-07-06

### Added

- Workspaces can set `settings.artifact_policy: block` to prevent `orc mark
  <ticket> next` from advancing while core feature docs or current-stage
  required artifacts are missing or empty.

## [0.7.0] - 2026-07-05

### Added

- Repos can declare `worktree_setup` in `orc.yaml`; `orc next` prints the resolved
  command when the target ticket worktree is missing, and `orc doctor` validates
  placeholders plus warns when the command omits `{{worktree_path}}`.
- Workflows can declare stage `required_artifacts`, repos can declare
  `agent_hints`, and validation now warns about stale feature-folder artifacts
  and recorded worktree drift.

## [0.6.0] - 2026-06-25

### Added

- `orc watch [ticket]` opens a compact, auto-refreshing rail of active agent
  work, with `--interval`, `--wide`, and `--tmux-toggle` (plus `--tmux-layout`
  and `--tmux-size`) for toggling a narrow watch pane in the current tmux window.

## [0.5.0] - 2026-06-19

### Added

- Release archives now publish `checksums.txt`, and the README documents shell
  completion generation.
- `orc doctor --system` checks install readiness outside a workspace, including
  version, PATH visibility, optional tools, tmux, chafa, and supported agent
  CLIs.
- `orc pack inspect <path>` validates and previews local workflow packs without installing them.
- `orc init --pack <path>` can scaffold a workspace from one local workflow pack.
- `orc init --skip-default-pack` creates the base scaffold without installing the default pack.
- `orc pack install <pack>` installs a built-in pack name or local pack path into an existing workspace.
- `orc pack list` and `orc pack show <pack>` report install provenance and active workflows.
- Pack composition checks now reject duplicate workflow IDs, duplicate worker
  IDs, duplicate stage IDs, and conflicting aliases before install.
- Generated workspaces now install the built-in default pack with aliases and a
  consistent namespaced runtime layout under `stages/<pack>/` and `workers/<pack>/`.

### Changed

- Makefile Go targets now use repo-local cache directories by default, avoiding
  home-cache permission failures in sandboxed runs.
- Pack documentation now clarifies embedded packs, installed snapshots, local
  path packs, workspace-owned files, and the roles of `inspect`, `install`,
  `list`, and `show`.

## [0.4.0] - 2026-06-18

### Added

- `orc init` now prints the setup, `cd`, and `orc doctor` next steps after real and dry-run workspace scaffolds.

## [0.3.3] - 2026-06-17

### Fixed

- Keep generated release notes outside the checkout so GoReleaser does not fail on a dirty worktree.

## [0.3.2] - 2026-06-17

### Added

- Repo-context guidance in the default workflow so intake records relevant local docs in `PLAN.md` and downstream stages use that context.
- Template lint coverage to catch stale workflow commands, missing worker references, and old review verdict spelling in embedded markdown.

### Changed

- Split the CLI command implementation into focused files by command area.
- Extracted common CLI output formatting helpers for status, mark, and archive output.

### Fixed

- Updated default stage and worker docs to use current `orc mark` command names, `needs-changes` verdict spelling, and correct QA closeout behavior.

## [0.3.1] - 2026-06-17

### Added

- A `workflow.description` field so workflows can document their intent.

### Changed

- Stage files now come from the workspace pack; the superseded `nextStep.md` was removed.

### Fixed

- `orc archive` now refuses non-completed tickets and checks the archive destination before cleanup.

## [0.3.0] - 2026-06-14

### Added

- Embedded workspace packs in `orc init`, with `--pack default` assumed when omitted.
- Golden manifests pinning `orc init` output, and a CI guard that every embedded pack is closure-complete.

### Changed

- Reworked the `SETUP.md` flow: self-verifying, batched questions per section, unified intake worker, and corrected readiness checks.
- Streamlined the README into `docs/reference.md` and documented release automation.

### Removed

- The `--with-sample-workers` flag, and dead worker frontmatter fields (`kind`, `default_tmux_window`).

## [0.2.0] - 2026-06-13

### Added

- `orc report` command for time-in-stage metrics.
- TUI health drill-in (a `doctor`-report view), features-list paging, scrollable detail view, and left/right stage navigation.

### Changed

- Resolve `next_action.cwd` against the workspace root consistently.
- Re-flow all file viewers on terminal resize.

### Fixed

- TUI crashes, dry-run state mutation, and status/count bugs.

## [0.1.0] - 2026-06-11

### Added

- Initial release: filesystem-based agentic workspace orchestration CLI.
- Tag-based versioning and a goreleaser release workflow.
- CI workflow running lint and test.
- Bubble Tea TUI dashboard with health, workflow, stage, and portrait views.
- `orc doctor --fix` to clear stale state locks.

[Unreleased]: https://github.com/cengebretson/orc/compare/v0.27.0...HEAD
[0.27.0]: https://github.com/cengebretson/orc/compare/v0.26.0...v0.27.0
[0.26.0]: https://github.com/cengebretson/orc/compare/v0.25.0...v0.26.0
[0.25.0]: https://github.com/cengebretson/orc/compare/v0.24.0...v0.25.0
[0.24.0]: https://github.com/cengebretson/orc/compare/v0.23.0...v0.24.0
[0.23.0]: https://github.com/cengebretson/orc/compare/v0.22.1...v0.23.0
[0.22.1]: https://github.com/cengebretson/orc/compare/v0.22.0...v0.22.1
[0.22.0]: https://github.com/cengebretson/orc/compare/v0.21.0...v0.22.0
[0.21.0]: https://github.com/cengebretson/orc/compare/v0.20.0...v0.21.0
[0.20.0]: https://github.com/cengebretson/orc/compare/v0.19.0...v0.20.0
[0.19.0]: https://github.com/cengebretson/orc/compare/v0.18.0...v0.19.0
[0.18.0]: https://github.com/cengebretson/orc/compare/v0.17.0...v0.18.0
[0.17.0]: https://github.com/cengebretson/orc/compare/v0.16.1...v0.17.0
[0.16.1]: https://github.com/cengebretson/orc/compare/v0.16.0...v0.16.1
[0.16.0]: https://github.com/cengebretson/orc/compare/v0.15.1...v0.16.0
[0.15.1]: https://github.com/cengebretson/orc/compare/v0.15.0...v0.15.1
[0.15.0]: https://github.com/cengebretson/orc/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/cengebretson/orc/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/cengebretson/orc/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/cengebretson/orc/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/cengebretson/orc/compare/v0.10.1...v0.11.0
[0.10.1]: https://github.com/cengebretson/orc/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/cengebretson/orc/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/cengebretson/orc/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/cengebretson/orc/compare/v0.7.2...v0.8.0
[0.7.2]: https://github.com/cengebretson/orc/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/cengebretson/orc/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/cengebretson/orc/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/cengebretson/orc/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/cengebretson/orc/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/cengebretson/orc/compare/v0.3.3...v0.4.0
[0.3.3]: https://github.com/cengebretson/orc/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/cengebretson/orc/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/cengebretson/orc/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/cengebretson/orc/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/cengebretson/orc/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/cengebretson/orc/releases/tag/v0.1.0
