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
- TUI and watch views, exact tmux pane targeting, attention-driven focus, live
  Claude/Codex telemetry, session search/resume, park/unpark recovery,
  repository grouping, and context-pressure presentation.

The following are explicitly post-v1 and do not block a release:

- Direct prompt sending from `orc watch` and structured human-response forms.
- Agent completion notifications.
- Durable arbitrary labels and label filters.
- Remote pack install/update/uninstall, registries, signing, and provenance.
- User profiles, hosted services, remote tmux control, cost attribution, and a
  Homebrew tap.

## 1. Automated gates

From the repository root:

```sh
make check
go test -race ./...
go mod tidy -diff
git diff --check
make build
./orc version
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

## 2. Fresh-workspace core QA

Build the candidate first, then keep its absolute path while testing outside the
repository:

```sh
ORC_BIN="$(pwd)/orc"
QA_ROOT="$(mktemp -d /private/tmp/orc-release-qa.XXXXXX)"

"$ORC_BIN" --workspace "$QA_ROOT" init --force
```

Verify the scaffold reports 40 created files and includes `SETUP.md`. In the
new workspace, let an agent follow the same setup path users receive:

```sh
cd "$QA_ROOT"
codex "Read SETUP.md and perform the workspace setup"
# or: claude "Read SETUP.md and perform the workspace setup"
```

Point setup at a disposable Git repository, then return to the Orc checkout or
use `$ORC_BIN` for every command:

```sh
"$ORC_BIN" --workspace "$QA_ROOT" doctor
"$ORC_BIN" --workspace "$QA_ROOT" work ORC-QA-1 --slug release-smoke
"$ORC_BIN" --workspace "$QA_ROOT" doctor ORC-QA-1
"$ORC_BIN" --workspace "$QA_ROOT" next ORC-QA-1 --dry
"$ORC_BIN" --workspace "$QA_ROOT" artifacts ORC-QA-1 --all

"$ORC_BIN" --workspace "$QA_ROOT" mark ORC-QA-1 start
"$ORC_BIN" --workspace "$QA_ROOT" mark ORC-QA-1 pause "release QA pause"
"$ORC_BIN" --workspace "$QA_ROOT" mark ORC-QA-1 resume
"$ORC_BIN" --workspace "$QA_ROOT" status ORC-QA-1 --json
"$ORC_BIN" --workspace "$QA_ROOT" mark ORC-QA-1 done --result "release QA complete"
"$ORC_BIN" --workspace "$QA_ROOT" archive ORC-QA-1
```

Confirm:

- Workspace `doctor` reports a valid config, workers, workflows, repository, and
  tools after agent-driven setup.
- The initial ticket `doctor` and `artifacts --all` report missing, empty, or
  unchanged scaffold artifacts and exit nonzero; this confirms incomplete
  templates cannot masquerade as finished work.
- `next --dry` prints an argv-based launch plan without starting a provider.
- Pause records the human-readable blocker; resume clears it.
- JSON status remains valid after every transition.
- Archive moves the completed feature under `features/_archive/`.

Repeat `init` once with `--skip-default-pack`; it should report 15 created files,
retain the agent-driven `SETUP.md`, and leave pack/workflow selection to setup.

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

- `orc watch` and `orc tui` show the durable ticket plus the optional live
  provider overlay.
- `/` filters by ticket, repository, branch, worker, engine, and provider model.
- `a` focuses the exact agent pane and `i` selects the next attention item.
- Context pressure is a percentage when the provider reports a limit and `n/a`
  otherwise.

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
4. Re-run sections 1–3 and commit the release preparation.

Verify the workflow contract before tagging:

```sh
version="$(cat VERSION)"
rg -n "^## \\[${version}\\]" CHANGELOG.md
git status --short --branch
git log -1 --oneline
```

The release commit must be pushed and CI on `main` must pass before the tag is
created.

## 5. Tag and verify artifacts

The release workflow runs for `v*` tags and rejects a tag that does not exactly
match `VERSION` or lacks a matching changelog section.

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

Download the archive for the current platform, verify it against
`checksums.txt`, then run:

```sh
./orc version
./orc doctor --system
```

The binary must report the tagged version. If publishing fails, fix the cause
and issue a new patch version; do not move or silently reuse a published tag.
