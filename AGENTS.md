# AGENTS.md

## Purpose

Orc is a Go CLI for filesystem-based agentic workspace orchestration. Durable
state belongs in files such as `STATE.yaml`; workflow policy belongs in
`orc.yaml`, markdown instructions, and worker definitions. Orc parses, validates,
renders, and updates those files while enforcing generic transition safety.

This is the shared repository guide. Codex reads it directly; `CLAUDE.md` imports
it for Claude. Keep product-specific guidance out of this file unless it is
explicitly scoped.

## Authoritative References

- `CONTEXT.md` — domain glossary and durable terminology decisions. Read it before
  introducing or changing domain terms, and update it with definition changes.
- `README.md` — installation, user workflows, and the complete CLI overview.
- `docs/reference.md` — workspace layout, file contracts, configuration, state,
  workers, and historical design divergences.
- `docs/workflows.md` — workflow configuration details.
- `docs/release.md` — release validation and publishing procedure.
- `docs/adr/` — load-bearing architectural decisions.
- `plan.md` — unshipped work only.

Start with the files being changed and these targeted references; do not load all
documentation by default.

## Architecture Boundaries

- `cmd/orc/` owns Cobra commands, flags, prompts, and terminal/JSON output. Keep
  command handlers thin.
- `internal/config`, `state`, `ticket`, `workers`, and `stage` own durable inputs.
- `internal/workspace`, `orchestrator`, `runner`, `resume`, and `parking` own
  workspace and lifecycle behavior.
- `internal/mux` is the terminal-multiplexer seam; `tmux` and `herdr` are backend
  implementations. Keep backend-specific behavior behind that interface.
- `internal/agentidentity`, `agenthooks`, `agentdetect`, and `telemetry` own agent
  identity, lifecycle publication, and conservative fallback detection.
- `internal/featurelist`, `ticketview`, `sessionlist`, and `workspacesnapshot` own
  shared read models. UI packages should consume those rather than rebuild them.
- `internal/dashboard`, `watch`, `workspaceui`, `sessionpicker`, and `ui` own
  terminal presentation.
- `internal/workspace/templates/` is embedded with `//go:embed all:templates`;
  retain `all:` so underscore-prefixed template directories are included.

## Core Invariants

- Orc and generated workspaces must work equally well for Claude and Codex.
  Product-specific behavior must be selected through the worker `engine`.
- Policy lives in workspace files, not command handlers.
- All state mutations go through `state.Update` for locking and atomic replacement.
- Runtime identity comes from durable state. Use recorded exact targets when
  present; fall back to legacy names only for backward compatibility.
- Keep human confirmation, recovery, and observability ahead of background
  automation.
- Keep CLI concerns at the boundary and reusable behavior in tested internal
  services.
- Do not duplicate shared instructions in `CLAUDE.md`; it must remain an import of
  this file.

## Working Rules

- Add every user-facing change to `CHANGELOG.md` under `## [Unreleased]` in the
  same change. Use `Added`, `Changed`, `Fixed`, or `Removed` as appropriate.
- Remove shipped work from `plan.md` when preparing the release that ships it.
- Cover user-visible command behavior, JSON shapes, validation, and dry-run prompts
  in `cmd/orc/main_test.go`.
- Put shared behavior tests beside the owning package rather than expanding command
  handlers.
- When changing generated workspace behavior, update the embedded templates and
  test the generated result.
- Preserve existing user changes and keep patches focused.

## Commands and Validation

```bash
make build          # build ./orc with version metadata
make test           # go test ./...
make lint           # golangci-lint
make check          # lint + test
make fmt            # gofmt -w
make tidy           # go mod tidy
make verify-agents  # install and exercise Claude/Codex lifecycle hooks
make release-check  # non-publishing release validation
```

Run the checks matching the changed files:

- Go: `make fmt`, targeted package tests, then `make test` when behavior spans
  packages. Run `make lint` for substantive Go changes.
- Module changes: `make tidy` and verify `go.mod`/`go.sum` are intentional.
- Shell scripts: run `shellcheck` on changed scripts.
- Agent hook or installer changes: run `make verify-agents`.
- Release plumbing: run `make release-check`.

Install the repository pre-commit hook once with
`ln -sf ../../scripts/pre-commit .git/hooks/pre-commit`; it runs tidy, format,
lint, and tests.

## Releases

Releases are automated and governed by `docs/release.md`:

1. Keep `CHANGELOG.md` current while changes land.
2. At release time, prune shipped work from `plan.md`.
3. Use `git release patch|minor|major` (or an explicit version), then push the
   resulting commit and tag with `git push origin main --follow-tags`.
4. Never create the GitHub release manually with `gh release create`; the tag
   workflow and GoReleaser own release creation and assets.

Use a minor bump for 0.x features and a patch bump for fixes. Run
`make release-check` before publishing.
