# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- `orc validate` no longer rejects a workspace-level `next_action.cwd` (such as
  `"."`) when the feature also has a worktree. Workspace-level stages like
  qa-automation intentionally run at the workspace root, which `ResolveCWD`
  already blesses; validation now agrees with it.

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

[Unreleased]: https://github.com/cengebretson/orc/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/cengebretson/orc/compare/v0.4.0...v0.5.0
[0.5.0]: https://github.com/cengebretson/orc/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/cengebretson/orc/compare/v0.3.3...v0.4.0
[0.3.3]: https://github.com/cengebretson/orc/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/cengebretson/orc/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/cengebretson/orc/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/cengebretson/orc/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/cengebretson/orc/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/cengebretson/orc/releases/tag/v0.1.0
