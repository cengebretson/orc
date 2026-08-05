# Release readiness

This runbook is the acceptance gate for an Orc release. Run it from a clean
checkout of the commit that will be tagged. Use a disposable repository and
workspace for manual QA; never point the release smoke test at active work.

## v1 scope

The v1 release boundary includes:

- Agent-driven workspace setup through `SETUP.md`, workspace validation, and
  exact generated-workspace golden coverage.
- Durable `STATE.yaml` workflow transitions, artifact policy, worktree setup,
  agent launch, JIT tasks, archive/delete, reporting, and local workflow packs.
- Dashboard workspace views and the dedicated Live rail, exact tmux pane
  targeting, attention-driven focus, live Claude/Codex telemetry, session
  search/resume, park/unpark recovery, repository grouping, and context-pressure
  presentation, including explicitly confirmed exact-agent prompting.
- Structured confirm/choice/text human responses, durable labels and label
  filters, per-worker time attribution, and terminal-palette theming.

The following are explicitly post-v1 and do not block a release:

- Remote pack install/update/uninstall, registries, signing, and provenance.
- User profiles, hosted services, remote tmux client selection, per-run
  post-mortem logs, token and currency cost, and a Homebrew tap.

## 1. Automated gates

From the repository root:

```sh
make check
go test -race ./...
go mod tidy -diff
git diff --check
make build
./orc version
make release-check
```

All commands must exit zero. The following formatting check must print nothing,
and `git status --short` must remain empty after validation:

```sh
rg --files -0 cmd internal -g '*.go' | xargs -0 gofmt -l
```

The workspace contract has dedicated golden tests. Run them explicitly so a
release reviewer can see both scaffold variants:

```sh
go test -count=1 ./internal/workspace/... \
  -run 'TestInit_Golden(DefaultPack|SkipDefaultPack)' -v
```

`make release-check` uses the GoReleaser version pinned in `.tool-versions`,
which is also consumed by both GitHub workflows. Locally, `mise` installs that
exact binary when needed. The target runs `goreleaser check` followed by a
snapshot release; snapshot mode evaluates builds, linker and archive templates,
release notes, archives, and checksums while explicitly skipping publication.
Generated artifacts remain under ignored `dist/` for inspection.

## 2. Fresh-workspace and artifact QA

Run the deterministic, noninteractive release smoke test:

```sh
make release-check
```

The target first verifies that `VERSION` has a non-empty matching changelog
section and comparison links. It then builds the candidate, creates disposable
workspaces and a disposable Git repository under the system temporary
directory, and verifies:

- Default initialization reports exactly 42 created files and includes
  `SETUP.md`.
- The deterministic post-agent setup contract produces a valid repository,
  workflow, worker, and tool configuration.
- Workspace and initial-ticket `doctor` checks pass, including the configured
  first-stage worker in `STATE.yaml.next_action.worker`.
- `artifacts --all` reports unchanged or missing scaffold artifacts and exits
  nonzero, so incomplete templates cannot masquerade as finished work.
- `next --dry` prints an argv-based launch plan without starting a provider.
- Pause records the human-readable blocker; resume clears it.
- JSON status remains valid after every transition.
- Archive moves the completed feature under `features/_archive/`.
- `--skip-default-pack` reports exactly 15 created files, retains `SETUP.md`,
  and does not install the default workflow.
- GoReleaser accepts the complete configuration and supplied release notes.
- Snapshot builds produce exactly one archive for macOS and Linux on amd64 and
  arm64, plus `checksums.txt`, without publishing a GitHub release.

The target intentionally does not launch Claude or Codex. When `SETUP.md`,
`AGENTS.md`, `RULES.md`, `TOOLS.md`, or the setup flow changes materially,
also perform the user-facing agent check:

```sh
ORC_REPO_ROOT="$(pwd)"
QA_ROOT="$(mktemp -d /private/tmp/orc-release-qa.XXXXXX)"
"$ORC_REPO_ROOT/orc" --workspace "$QA_ROOT" init --force
cd "$QA_ROOT"
PATH="$ORC_REPO_ROOT:$PATH" codex "Read SETUP.md and perform the workspace setup"
# or: PATH="$ORC_REPO_ROOT:$PATH" claude "Read SETUP.md and perform the workspace setup"
```

## 3. Live session QA

This section starts and stops a real provider session. Run it only in the
disposable QA workspace and make sure no unrelated tmux sessions share its
ticket/session name.

```sh
"$ORC_BIN" --workspace "$QA_ROOT" work ORC-QA-2 --slug live-session --tmux
"$ORC_BIN" --workspace "$QA_ROOT" next ORC-QA-2
"$ORC_BIN" --workspace "$QA_ROOT" sessions
"$ORC_BIN" --workspace "$QA_ROOT" status ORC-QA-2 --json
```

In a second terminal, verify:

- `orc watch` and `orc dashboard` show the durable ticket plus the optional live
  provider overlay.
- `/` filters by ticket, repository, branch, worker, engine, and provider model.
- `a` focuses the exact agent pane and `i` selects the next attention item.
- `s` opens prompt composition; `enter` only advances to review, `n` cancels,
  and only an explicit `y` sends to the captured exact agent instance.
- Context pressure is a percentage when the provider reports a limit and `n/a`
  otherwise.

With lifecycle hooks healthy, send a disposable prompt containing spaces,
quotes, shell metacharacters, and a newline through `orc ctl agent prompt
--wait`. Confirm the text arrives literally in only the recorded pane and the
command returns a recognized lifecycle or a distinct structured stall,
replacement, exit, cancellation, or timeout error.

Exercise recovery after the provider has emitted a resumable session identity:

```sh
"$ORC_BIN" --workspace "$QA_ROOT" sessions park --dry
"$ORC_BIN" --workspace "$QA_ROOT" sessions park --yes
"$ORC_BIN" --workspace "$QA_ROOT" sessions unpark --dry
"$ORC_BIN" --workspace "$QA_ROOT" sessions unpark --yes
"$ORC_BIN" --workspace "$QA_ROOT" sessions
```

Confirm that dry runs do not mutate tmux or snapshots, park leaves unrelated
sessions alone, unpark restores the exact provider identity, and a second retry
reconciles an already-restored matching pane without adopting an unrelated tmux
name collision. Also copy one provider session ID from `sessions --json` and
verify `sessions resume <id> --dry` prints the expected provider argv and CWD.

## 4. Prepare the release commit

Choose the version, then update all three release inputs in one focused commit:

1. Set `VERSION` to the version without the `v` prefix.
2. Move the relevant entries from `[Unreleased]` into
   `## [<version>] - YYYY-MM-DD` and leave a fresh `[Unreleased]` section.
3. Update the comparison links at the bottom of `CHANGELOG.md`: `[Unreleased]`
   starts at the new tag, and `[<version>]` compares the previous tag to it.
4. Delete anything from `plan.md`'s backlog that this release ships. Nothing
   automates this, and it is the step most often missed: a shipped item left in
   the backlog is the one piece of stale documentation a reader has no way to
   detect, because a roadmap entry looks the same whether or not it is done.
5. Re-run sections 1–3 and commit the release preparation.

Verify the workflow contract before tagging:

```sh
version="$(cat VERSION)"
make release-check
RELEASE_TAG="v${version}" ./scripts/release-contract
git status --short --branch
git log -1 --oneline
```

The release commit must be pushed and CI on `main` must pass before the tag is
created.

## 5. Tag and verify artifacts

The release workflow runs for `v*` tags, installs the exact GoReleaser version
from `.tool-versions`, and rejects a tag that does not exactly match `VERSION`
or lacks a matching, non-empty changelog section and comparison links.

```sh
version="$(cat VERSION)"
git tag -a "v${version}" -m "orc v${version}"
git push origin "v${version}"
```

After the workflow succeeds, verify the GitHub release contains:

- `orc_<version>_darwin_amd64.tar.gz`
- `orc_<version>_darwin_arm64.tar.gz`
- `orc_<version>_linux_amd64.tar.gz`
- `orc_<version>_linux_arm64.tar.gz`
- `checksums.txt`
- Release notes matching the `CHANGELOG.md` section for the version.

The workflow's final step asserts that last item through
`scripts/verify-release-published`, comparing the published body against the
notes GoReleaser was handed. It is the only check that can: `make release-check`
runs GoReleaser in snapshot mode, and snapshot mode never renders a release
body, so no local gate can see this. A failure there means the release is
already public — repair it with:

```sh
gh release edit "v${version}" --notes-file <notes>
```

`scripts/release-contract` guards the known cause up front, before a tag
exists: it rejects a `.goreleaser.yaml` that sets `changelog.disable`, because
GoReleaser reads `--release-notes` inside its changelog pipe and disabling the
pipe drops the notes silently. Every release through `v0.17.0` published with
an empty body for that reason.

Download the archive for the current platform, verify it against
`checksums.txt`, then run:

```sh
./orc version
./orc doctor --system
```

The binary must report the tagged version. If publishing fails, fix the cause
and issue a new patch version; do not move or silently reuse a published tag.
