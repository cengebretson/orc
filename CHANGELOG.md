# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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

[Unreleased]: https://github.com/cengebretson/orc/compare/v0.16.1...HEAD
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
