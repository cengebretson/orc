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

`CONTEXT.md` is the authoritative glossary for Orc's own domain vocabulary. Read
it before introducing or changing domain terms, and update it in the same change
when a definition changes.

## Repository Layout

```
orc/
  cmd/orc/                        CLI entry point and command handlers (Cobra)
  internal/
    ── workflow and durable state ──
    config/                       orc.yaml parsing — repos, workflows, loop stages, settings
    state/                        STATE.yaml parsing and atomic mutations
    ticket/                       ticket lookup and load helpers
    workers/                      worker definition parsing
    stage/                        stage markdown file reading
    workspace/                    init, work, and template embedding
      templates/                  embedded workspace scaffold templates
    workspacectx/                 loads a workspace root into a shared Context (config + workers), with a validated variant
    orchestrator/                 launch, transition, and archive services
    runner/                       next-action resolution — worker, prompt, launch args
    resume/                       recovery prompt builder
    parking/                      reversible park/restore policy for live sessions
    ── validation and health ──
    validate/                     per-ticket state validation
    doctor/                       workspace + local tool readiness checks
    health/                       workspace filesystem health checks
    artifactcheck/                stage artifact presence and change detection
    worktreecheck/                worktree ownership and drift checks
    worktreesetup/                worktree creation and repo wiring
    ── multiplexer seam ──
    mux/                          the terminal multiplexer seam (Backend interface, Target, Metadata)
      muxtest/                    fake backend for tests
    tmux/                         tmux backend — sessions, panes, metadata, watch rail
    herdr/                        Herdr backend using the herdr CLI
    ── agent identity and lifecycle ──
    agentidentity/                opaque IDs for durable agents and live instances
    agenthooks/                   installs/inspects Orc-owned Codex and Claude lifecycle hooks
    agentdetect/                  conservative, presentation-only terminal fallback rules
    telemetry/                    provider session discovery (Claude/Codex JSONL)
    ── read models ──
    featurelist/                  shared feature collection for CLI status and Workspace rows
    ticketview/                   single-ticket display/runtime summary
    sessionlist/                  joins durable state with live sessions — managed/orphan/unmanaged
    workspacesnapshot/            one immutable view of the workspace for a render pass
    report/                       time-in-stage derivation from STATE.yaml history
    gitmeta/                      bounded, best-effort repository metadata
    contextpressure/              classifies provider context usage for presentation
    ── terminal UI ──
    dashboard/                    composes watch (Live) and workspaceui into one Bubble Tea shell
    watch/                        the Live watch rail model
    workspaceui/                  Features/Workflows/Workers/Repositories/Health models
    sessionpicker/                interactive provider-session chooser
    ui/                           terminal presentation primitives shared by the above
    searchmatch/                  shared presentation-only filter matching
    notify/                       optional user-configured command run after transitions
  scripts/
    pre-commit                    tidy → fmt → lint → test (symlink to .git/hooks/pre-commit)
    release-check                 non-publishing release validation (make release-check)
  docs/
    workflows.md                  workspace workflow configuration reference
    reference.md                  deep reference — layout, files, orc.yaml, STATE.yaml, workers
    sessions.md, tmux.md, herdr.md, watch.md, agent-detection.md, release.md
  CONTEXT.md                      authoritative glossary for Orc's domain vocabulary
  go.mod
  Makefile
  plan.md                         roadmap — unshipped work only
```

## Dev Workflow

```bash
make build   # go build -o orc ./cmd/orc/...
make test    # go test ./...
make lint    # golangci-lint (errcheck, govet, staticcheck, unused, ineffassign)
make check   # lint + test together
make fmt     # gofmt -w
make tidy    # go mod tidy
make install # go install ./cmd/orc/... (with version LDFLAGS)
make clean   # rm -f orc
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

### Keep the changelog current (do this as you work)

**Every user-facing change adds a bullet to the `## [Unreleased]` section of
`CHANGELOG.md`, in the same commit/PR that makes the change.** Group bullets
under `### Added` / `### Changed` / `### Fixed` / `### Removed`. This is the one
part nobody automates: `git release` promotes and dates the section but does
**not** author the notes — write them as work lands so release time is trivial.
At release time, if `[Unreleased]` was missed, reconstruct it from
`git log <last-tag>..HEAD` before cutting.

### Cutting a release

Use the repo-agnostic `git release` helper (`~/.local/bin/git-release`, shared
across the tmux plugins):

```bash
git release minor          # or: patch | major | an explicit X.Y.Z
git push origin main --follow-tags
```

`git release` bumps `VERSION`, promotes `## [Unreleased]` to
`## [X.Y.Z] - <date>` (adding a fresh empty `[Unreleased]` and rewriting the
compare links), runs `make test`, commits "Release vX.Y.Z", and tags — it does
**not** push, and it **refuses** to release if `[Unreleased]` is empty (override
with `--allow-empty-changelog`). Preview with `git release minor --dry-run`. Add
`-p`/`--push` to push automatically.

Pushing the `v*` tag triggers `.github/workflows/release.yml`, which: verifies
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
