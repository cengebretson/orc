# CLAUDE.md

## Project Overview

**orc** is a Go CLI for agentic workspace orchestration. It scaffolds and manages a
filesystem-based workspace where agents (Claude, Codex, Cursor, etc.) carry feature
work across repos — from ticket intake through implementation, PR repair, QA automation,
and evidence collection.

The core idea: durable state lives in files (`STATE.yaml`, markdown docs), not in memory.
Workflow policy lives in files (`orc.yaml`, `RULES.md`, `AGENTS.md`, worker definitions),
not in command handlers. `orc` enforces generic state transitions and safety rules,
then reads, validates, renders, and updates those files.

## Repository Layout

```
orc/
  cmd/orc/main.go                 CLI entry point (Cobra)
  internal/
    config/                       orc.yaml parsing — repos, workflows, loop stages, settings
    doctor/                       workspace + local tool readiness checks
    featurelist/                  shared feature collection for CLI status and TUI rows
    report/                       time-in-stage derivation from STATE.yaml history
    health/                       workspace filesystem health checks
    orchestrator/                 launch, transition, and archive services
    runner/                       next-action resolution — worker, prompt, launch args
    state/                        STATE.yaml parsing and mutations
    ticket/                       ticket lookup and load helpers
    ticketview/                   single-ticket display/runtime summary
    workers/                      worker definition parsing
    stage/                        stage markdown file reading
    resume/                       recovery prompt builder
    validate/                     per-ticket state validation
    tmux/                         tmux session management
    tui/                          Bubble Tea dashboard
    workspace/                    init, work, and template embedding
      templates/                  embedded workspace scaffold templates
  scripts/
    pre-commit                    tidy → fmt → lint → test (symlink to .git/hooks/pre-commit)
  docs/
    workflows.md                  workspace workflow configuration reference
    reference.md                  deep reference — layout, files, orc.yaml, STATE.yaml, workers
  go.mod
  Makefile
  plan.md                         roadmap — up-next work, future ideas, cleanup, done record
```

## Dev Workflow

```bash
make build   # go build -o orc ./cmd/orc/...
make test    # go test ./...
make lint    # golangci-lint (errcheck, govet, staticcheck, unused, ineffassign)
make check   # lint + test together
make fmt     # gofmt -w
make tidy    # go mod tidy
```

Install the pre-commit hook once after cloning:

```bash
ln -sf ../../scripts/pre-commit .git/hooks/pre-commit
```

The hook runs tidy → fmt → lint → test on every commit.

## Releases

Releases are **automated** — do not create them by hand. The release ritual
matches the sibling tmux repos: a `VERSION` file and a `CHANGELOG.md` drive
the release, and a `v*` tag triggers it.

To cut a release:

1. Bump `VERSION` to the new number (no `v` prefix, e.g. `0.4.0`).
2. In `CHANGELOG.md`, rename the `## [Unreleased]` section to
   `## [0.4.0] - YYYY-MM-DD` and add a fresh empty `## [Unreleased]` above it.
3. Commit, then tag and push:

   ```bash
   git tag -a v0.4.0 -m "v0.4.0"
   git push origin v0.4.0
   ```

Pushing the tag triggers `.github/workflows/release.yml`, which: verifies
`VERSION` matches the tag, runs `go test ./...`, extracts the matching
`CHANGELOG.md` section into the release notes, then runs goreleaser
(`release --clean --release-notes=...`) to build the binaries and create the
GitHub Release with those notes. The workflow **fails** if `VERSION` doesn't
match the tag or there's no changelog section for it.

Never run `gh release create` or otherwise create the release manually — it
collides with goreleaser. After pushing the tag, watch the Release workflow in
the Actions tab; the release and its assets appear when goreleaser finishes.
Version numbers follow 0.x semver (minor bump for features, patch for fixes).
`make build` stamps the binary version from the latest git tag.

## Commands

See `README.md` for the full command reference split into human and agent commands.
Quick reference for dev/test use:

```bash
./orc init --dry-run
./orc init --list-packs
./orc init --workspace /tmp/test-ws --pack default
./orc doctor --workspace /tmp/test-ws
./orc work STORY-123 --workspace /tmp/test-ws
./orc next STORY-123 --dry --workspace /tmp/test-ws
./orc next STORY-123 --json --workspace /tmp/test-ws
./orc status STORY-123 --workspace /tmp/test-ws
./orc jit STORY-123 --worker bob-developer "check the handoff" --dry --workspace /tmp/test-ws
```

Keep command-level behavior covered in `cmd/orc/main_test.go` when changing user-facing
output, JSON shape, validation behavior, or dry-run prompts. Use package tests for shared
services (`internal/orchestrator`, `internal/ticket`, `internal/ticketview`,
`internal/featurelist`, `internal/state`) so command handlers stay thin.

## Template System

Templates are embedded in the binary via `//go:embed all:templates`.
The `all:` prefix is required to include directories starting with `_` (like `_template`).

To add a new template file, drop it under `internal/workspace/templates/` and rebuild.

## Hard Requirements

**orc and the workspaces it generates must work equally well for Claude and Codex.**

- The workspace scaffold must be readable and actionable by both without modification.
- `CLAUDE.md` imports `AGENTS.md` as the shared source of truth. Codex reads `AGENTS.md`
  directly. The two must never diverge.
- `orc` CLI output must be correct for whichever engine the worker specifies — never
  assume Claude.
- Do not add features or template content that only makes sense for one product.
  Gate product-specific behavior behind the worker's `engine` field at runtime.

## Design Principles

- **Policy in files, not code.** Worker behavior, model choice, cost tier, and
  workflow routing live in markdown files and `orc.yaml`. `orc` parses, matches,
  renders, updates state, and enforces generic transition safety.
- **Durable state.** `STATE.yaml` survives restarts, session changes, and agent switches.
- **Atomic state writes.** All state mutations go through `state.Update`, which locks,
  writes a temp file, and atomically replaces `STATE.yaml`. Stale dead-PID locks are
  recoverable; `orc doctor` reports lock files.
- **Runtime identity lives in state.** Use `runtime.tmux.session` when present; fall back
  to slug only for older tickets. Do not assume tmux session name always equals `slug`.
- **Human-in-the-loop first.** Background execution comes after logging and recovery are
  solid.
- **Stage-assigned workers by default.** Override with `--worker` for a single run or
  set `stage.worker` via `orc mark <ticket> next --worker` to persist across sessions.
- **Product-agnostic by default.** Every decision that could couple `orc` to a single
  agent product should be reconsidered.
- **CLI as the boundary, services as the source of behavior.** Keep prompts, flags, and
  printing in `cmd/orc`; put orchestration, ticket lookup, runtime summaries, state
  transitions, and archive behavior in internal packages with tests.

## Deliberate Divergences from Original Design

| Original | What we did instead | Why |
|----------|--------------------|----|
| `JIRA.md` in feature template | `TICKET.md` | System-agnostic — works with GitHub Issues, Linear, local files, or manual |
| `django/` subfolder in features | per-stage subfolders (`develop/`, `code-review/`, etc.) | Each stage writes to its own named folder — provenance is unambiguous |
| `orc workon` command | `orc work` | Shorter, cleaner |
| `orc done` command | `orc archive` with `_archive/` folder | Preserves history, keeps workspace clean, reversible |
| No first-run config | `SETUP.md` agent-driven setup | Cleaner than hand-editing files; works with Claude or Codex |
| Intake bundled into main workflow | Separate `intake` stage | Cleaner separation — every ticket goes through intake first |
| `backend/` subfolder | per-stage subfolders | Stage name is the folder name — self-documenting and not coupled to any stack |
| Worker `stages:` / `workflows:` fields | Routing lives entirely in `orc.yaml` | Single source of truth; explicit errors when no worker assigned |
