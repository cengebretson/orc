# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

- Embedded workspace packs in `orc init`, with `--pack default` assumed when omitted and `--pack none` as an escape hatch.
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

[Unreleased]: https://github.com/cengebretson/orc/compare/v0.3.2...HEAD
[0.3.2]: https://github.com/cengebretson/orc/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/cengebretson/orc/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/cengebretson/orc/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/cengebretson/orc/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/cengebretson/orc/releases/tag/v0.1.0
