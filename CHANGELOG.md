# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/cengebretson/orc/compare/v0.8.0...HEAD
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
