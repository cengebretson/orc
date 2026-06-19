# orc — Plan

Single roadmap file. **Up next** holds specced, ready-to-build work;
**Future ideas** are unspecced and lower priority; **Cleanup** is open
hardening items; the **Done** record and **Considered and rejected** keep
decisions from being relitigated.

---

## Up next

### Agent completion notification

Fire a user-defined shell command when a ticket transitions state. Most useful
in `--tmux` mode where sessions run unattended and the user needs a signal that
work is ready for review or is blocked.

look at slackcat for an example: `slackcat --channel alerts --username Grommash -m "The build is complete, warchief."`

#### Config shape (`orc.yaml`)

```yaml
settings:
  notify:
    on: [blocked, complete]          # events: blocked | complete | error | all
    command: "notify-send 'orc' '{{ticket}} {{event}}'"
```

Event data is exported as environment variables (`ORC_TICKET`, `ORC_SLUG`,
`ORC_EVENT`, `ORC_STAGE`, `ORC_WORKFLOW`) and the command runs as-is — no
string splicing into shell, so quoting can never break. `{{var}}` template
expansion is kept as sugar for the same five values:
- `{{ticket}}` — ticket ID (e.g. `STORY-123`)
- `{{slug}}` — full feature slug (e.g. `STORY-123-add-login`)
- `{{event}}` — event name (`blocked`, `complete`, `error`)
- `{{stage}}` — stage name at the time of the event
- `{{workflow}}` — workflow name

#### Events

| Event | When it fires |
|-------|--------------|
| `complete` | ticket advances to next stage (or done if final) |
| `blocked` | `orc mark <ticket> pause` is called |
| `error` | future — agent explicitly signals failure |
| `all` | shorthand for all of the above |

#### Implementation notes

- Add `NotifySettings` struct to `internal/config` with `On []string` and `Command string`
- Add `Notify NotifySettings yaml:"notify"` to `Settings`
- Add `internal/notify` package: `Fire(cfg *config.NotifySettings, event, ticket, slug, stage, workflow string)` — sets `ORC_*` env vars, expands template vars, checks `On` list, runs command via `os/exec` with a short timeout
- Call `notify.Fire` in `runMarkNext` and in `runMark` for both the **pause** and **done** cases after state is written — `orc mark <ticket> done` is its own switch arm and must fire `complete` too, not just the advance path
- No-op when `command` is empty or event not in `on` list

**Effort:** Medium.

---

### Workspace packs (embedded) — ✅ shipped 2026-06-14

> Shipped in commit 216335c (mechanism) plus a CI closure guard. The detail
> below is retained as the as-built record — it matches what was built, with
> these notes:
> - **orc.yaml assembly is text-based, not a yaml round-trip.** A round-trip
>   would drop the base file's comments and reformat it, so a single-pack
>   install would no longer reproduce the hand-written orc.yaml byte-for-byte.
>   The first pack carries the `workflows:` header verbatim; later packs
>   contribute their indented body. Byte-fidelity is pinned by
>   `init_default.manifest`.
> - **Post-install closure was already enforced** by `orc doctor`'s
>   pre-existing "workflow refs" check (worker-id→file, stage→file, loop refs).
>   `TestEmbeddedPacks_ClosureComplete` runs that exact check against every
>   embedded pack at CI time, so a pack-authoring mistake fails `make test`.
> - **Deferred** until a second pack exists: multi-pack conflict detection
>   (duplicate workflow name / worker id; divergent same-name stage file). Only
>   a duplicate worker *filename* is guarded today. Untestable without pack #2.

Ship curated bundles of policy files so `orc init` can scaffold a ready-to-run
workflow instead of the single hardcoded default. Embedded and offline first;
the on-disk format is `tar`-able so a remote registry is a later additive step,
not a reshape.

#### What a pack is

The scaffold splits into two groups. **Structural** files never travel — they
define the workspace contract and are identical everywhere: `AGENTS.md`,
`CLAUDE.md`, `RULES.md`, `ORC.md`, `ROUTER.md`, `TOOLS.md`, `SETUP.md`,
`features/_template/`, `workers/_template.md`, `.gitignore`.

**Policy** is the pack — the three entangled things that must travel together:
a workflow block, every worker its stages route to (`worker: default:bob`),
and every stage file those stages read (`stages/develop.md`). A pack is exactly
the closure of "a workflow + its workers + its stage files." Shipping less than
the full triple produces broken half-installs (a stage pointing at a worker or
`stages/*.md` that does not exist).

Workers travel well; workflows are entangled with the user's repos and stage
conventions, so a pack's workflow is a starting template the user edits, never a
turnkey config.

#### Template layout

```
templates/
  _base/                      # always installed (structural group)
    AGENTS.md CLAUDE.md RULES.md ORC.md ROUTER.md TOOLS.md SETUP.md
    orc.yaml                  # settings + repos stub + EMPTY workflows:
    features/README.md features/_template/*
    workers/_template.md
  packs/
    default/                  # today's sample workflow, promoted to a real pack
      pack.yaml
      workers/*.md            # bob, zach, brian, fred, ada
      workflow.yaml           # ONE workflows: block (current orc.yaml lines 41-66)
      stages/*.md             # intake develop code-review pr-open pr-repair qa-automation
    go-backend/
      pack.yaml workers/ workflow.yaml stages/
```

`pack.yaml` manifest — enables versioning, mixing, and cross-engine enforcement:

```yaml
name: default
description: General feature workflow — intake → develop → PR → QA
schema: 1                     # worker/workflow frontmatter schema version
engines: [claude, codex]      # declares cross-engine support (hard requirement)
provides:
  workflows:
    - id: default:standard
      path: workflow.yaml
  workers:
    - id: default:bob
      path: workers/bob.md
    - id: default:zach
      path: workers/zach.md
  stages:
    - id: default:develop
      path: stages/develop.md
```

#### CLI surface

```
orc init                      # assumes --pack default (interactive: prompts "Which pack? [default]")
orc init --skip-default-pack  # _base/ only — empty workflows:, author your own
orc pack install go-backend   # add a built-in pack to an existing workspace
orc pack install ./some-dir   # add a local pack to an existing workspace
orc pack list                 # show installed packs, provenance, and active workflows
orc pack show default         # inspect one installed pack snapshot
orc init --list-packs         # name + description + engines, read from each pack.yaml
```

When `--pack` is omitted, `default` is assumed (and is the pre-filled answer in
the interactive prompt). This also fixes a latent bug in today's no-flag init:
the base scaffold currently writes an `orc.yaml` whose `default` workflow routes
to workers it does not install (closure-broken out of the box). Because a pack
ships workflow + workers + stages as one closed set, the assumed default is
always runnable. `--skip-default-pack` is the explicit opt-out for authoring
from a clean base or installing packs later with `orc pack install`.

`--with-sample-workers` is **removed**, not aliased. It is a 0.x flag with no
external contract to honor, and `--pack default` fully subsumes it. Removal also
reshapes its interactive prompt: the boolean `Include sample workers? [y/N]`
(cmd/orc/main.go:373-376) becomes a pack chooser (`Which pack? [default]`),
which is the better UX anyway. The output oracle survives the flag's death —
`init_sample.manifest` (the 27-file scaffold) stays the source of truth and
`--pack default` must reproduce it byte-for-byte; only the test *invocation*
changes from `WithSampleWorkers: true` to the pack API. Code/doc touch points:
`InitOptions.WithSampleWorkers` + the `isSample` gating in init.go, the flag/
prompt/opts wiring in main.go, `TestInit_WithSampleWorkers`, and the README /
`docs/reference.md` / `CLAUDE.md` mentions.

#### Implementation notes

The one real change is to `internal/workspace/init.go`:

- **`collectEntries`** walks `_base/` always; selected packs are snapshotted
  into `packs/<name>/` and materialized into `workers/<name>/` and
  `stages/<name>/`.
- **`orc.yaml` stops being copied verbatim** — it is now assembled. Start from
  `_base/orc.yaml` (empty `workflows:`), splice each selected pack's
  `workflow.yaml` block under `workflows:`, set `settings.default_workflow` to
  the first pack's workflow if unset. A `gopkg.in/yaml.v3` round-trip replacing
  one `WriteFile`. Dry-run printer, force handling, and runtime-dir creation are
  untouched.
- **Conflict + closure checks at init.** Collision rules differ by name kind —
  the principle is that for anything semantically load-bearing a collision is an
  error, never a silent winner, because a silent winner means one pack's workflow
  runs with another pack's policy. Precedence and rules:
  - **Existing on-disk files win over fresh pack content**, unless `--force` —
    the existing skip-if-exists behavior (init.go:116) is the local-override
    layer (user edits beat packs).
  - **`_base/` vs packs: no overlap by construction** — base owns structural
    files, packs own `workers/` + `workflow.yaml` + `stages/`, so a pack can
    never shadow `RULES.md` etc. (Pack-level overrides of base files are the
    Future-ideas `overrides/` layer, deliberately out of v1.)
  - **Pack vs pack:** duplicate canonical workflow, worker, or stage IDs are a
    hard error. Duplicate aliases are allowed only when they point to the same
    canonical target.
  - **Ordered `--pack a --pack b` last-wins is rejected** — invisible precedence
    is a footgun; force explicit resolution instead. An opt-in override flag can
    come later, not as the default.
  - **Closure check** — every `worker:` named in a pack's `workflow.yaml` exists
    in its `workers/`, every stage name resolves to a `stages/*.md`. Fold the
    same closure check into `orc doctor` so a hand-edited installed workspace is
    caught too; `doctor` is also the cross-engine enforcement point (warn if a
    worker's `engine` is not in the pack's declared `engines`).

Explicitly **not** in scope: network, registry, remote fetch. Packs stay
`//go:embed`-ed; `orc init --pack go-backend` and `orc pack install go-backend`
are fully offline. The directory format (`pack.yaml` + `workers/` +
`workflow.yaml` + `stages/`) is `tar`-able so `orc pack install ./some-dir/` or
a remote registry is an additive change to *where packs are read from*, not a
format reshape.

#### Migration (one pass)

1. `git mv` structural files into `templates/_base/`; sample workers + current
   workflow + stages into `templates/packs/default/`.
2. Move the `workflows:` block out of `_base/orc.yaml` into
   `packs/default/workflow.yaml`.
3. Add `pack.yaml` to `default/`.
4. Rewrite `collectEntries` for base-always + selected-packs; add the orc.yaml
   assembler.
5. Remove `--with-sample-workers` and its `[y/N]` prompt; add `--pack` /
   `--list-packs` and the pack chooser.

Blast radius: `init.go` + the flag/prompt wiring in `cmd/orc/main.go` + a
template reshuffle. The `--pack default` output must stay byte-identical to
today's `--with-sample-workers` scaffold (pinned by `init_sample.manifest`), so
the golden test catches any drift even though the flag name is gone.

**Effort:** Medium.

---

## Future ideas

Lower priority — worth revisiting once the core is solid.

- **Per-run log capture** (`orc logs` / `orc tail`) — stream each launched
  agent's raw transcript to `<featureDir>/<stage>/run-<timestamp>.log` (tmux
  `pipe-pane`, or tee in foreground), record the path under `runtime`, expose
  it via two commands. *Deliberately deferred:* orc's durable record is the
  curated handoff — `DECISIONS.md`, `STATE.yaml` history, per-stage output —
  which is meant to be read; a raw transcript is process noise nobody parses.
  Its only real payoff is post-mortem debugging of an *unattended* run that
  failed, so it's contingent on background execution actually existing. Build
  it as the debug surface for background mode, not before.

- **Workspace packs — remote/team distribution** — embedded packs have landed
  (see Up next, shipped); grow into sharing across a team: `orc pack push/pull/diff` against a git
  repo, `orc init --pack ./dir/` or a URL, a two-layer model with `overrides/`
  for local customization, and a trust/lint story (`orc doctor` validates a
  fetched pack's schema + engines). The embedded format is deliberately
  `tar`-able so this is additive — *where* packs are read from, not a reshape.
- **Per-worker cost attribution** — build on `orc report`: history entries
  carry `worker`, worker definitions carry model/cost tier; roll stage
  durations up into per-worker time and estimated cost.
- **TUI event feed** — once `notify.Fire` lands (Up next above), also append
  events to a workspace `events.log`; the TUI tails it into a recent-activity
  pane. Cheap after notifications ship; pointless before.

---

## Cleanup

- [ ] (optional) `viewDashboard` composition remains untested (`viewDetail`
      now has a smoke test via the Timing section); golden-file snapshot tests
      are the cheapest way in if it ever matters.

---

## Considered and rejected

- `internal/stage` at 0% coverage — 36 trivial lines, not worth tests.
- `internal/tmux` at 23% — wraps the tmux binary; mocking it buys little.
- `cmd/orc/main.go` at 1,388 lines — it's flag definitions and printing, which is
  exactly what the "CLI as the boundary" principle prescribes. No action.
- Cobra package-level flag vars (`markStage` etc.) — standard cobra idiom.

---

## Done — roadmap items

### Stage timing — `orc report` (done 2026-06-12)

- [x] New `internal/report` package: `Compute(*state.State, now) Report` derives
      per-stage active/wall/visits from history (each entry stamps the stage
      active up to its time, so an interval is attributed to the closing entry's
      stage; the open current stage is measured to `now`). `Aggregate([]Report)`
      rolls up avg/median active + visit counts per stage. Pause→resume gaps are
      subtracted from active time; bad/out-of-order timestamps are skipped, never
      fatal. Terminal statuses (`done`, `archived`) get no open interval; a
      `paused` ticket's current-stage interval is frozen via the status field
      (authoritative over the free-form result string). Package coverage ~97%.
- [x] `orc report [ticket]`: no-arg aggregate table across tickets (`--archived`
      to include `_archive/`), `orc report <ticket>` single-ticket breakdown with
      total cycle time, both with `--json`. Thin handler in `cmd/orc`; tickets
      collected via `featurelist.Collect`. Command-level tests cover table,
      JSON shape, and aggregate.
- [x] TUI: the feature detail page gained a **Timing** section above History,
      rendering `report.Compute` (per-stage active/wall/visits + total, with a
      "← current" marker on the open stage). First test to exercise `viewDetail`
      directly.
- Defaults shipped as specced: active time is the headline (wall shown
  alongside), repair loops summed per stage with a visit count. The command
  was the first surface; the TUI Timing section followed. Per-worker cost
  attribution remains in Future ideas, building on this.

### TUI left/right stage navigation (done 2026-06-12)

- [x] Stage file viewer: `left`/`right` (`h`/`l`) jump to the previous/next
      stage's `.md` in pipeline order, driving `wfDetailCursor` and rebuilding
      the `step N of M` title (`loadViewerStage`, mirror of `loadViewerFile`).
      Esc back now scrolls the workflow detail so the cursor row stays visible
      (was restoring the file viewer's offset). Workflow detail page:
      `left`/`right` alias the stage cursor. Help text updated on both views.
      Coverage 55.5% → 58.1%.

---

## Done — 2026-06-10 review findings

Snapshot at review time: ~10.6K lines of Go across 18 packages; 14 of 18 at
60–89% test coverage. Design principles in CLAUDE.md verified against the
code: `cmd/orc` handlers are thin wrappers over
`internal/orchestrator`/`internal/state`, state mutations are atomic
(lock → mutate → temp file → rename), error wrapping is consistently `%w`,
zero TODO/FIXME debt.

### TUI test coverage and structure (gate met — done 2026-06-11)

The TUI was the highest-churn, lowest-coverage code: `internal/tui/tui.go` was
2,151 lines (~20% of all Go in the repo) and the package sat at **6.9% coverage**
vs 60–89% everywhere else. Four of the last five commits touched the TUI; the
portrait pixel regression (fixed in a90785f) is the class of bug coverage here
would catch. The gate was "do this before the rest of the TUI roadmap" —
coverage is now 54.1% with the file split done, so the roadmap is unblocked.
One soft follow-up remains under Cleanup above.

- [x] Split `tui.go` (2,151 lines) into six per-concern files: `model.go`,
      `update.go`, `data.go`, `render.go`, `view_dashboard.go`, `view_detail.go`.
      Pure file moves — declaration list verified identical before/after.
- [x] Added `render_test.go` covering `renderRouteChain` (incl. wrapping and loop
      annotations), `renderTable`, `renderWorkflowDetail`, `renderWorkerFile`,
      `visibleFeatures`, and the primitives (`padRight`, `truncate`, `wrapText`,
      `drawBoxLabeled` width invariant). Package coverage: 6.9% → 28.3%.
- [x] Added `update_test.go` driving `handleKey`/`Update` directly with
      `tea.KeyMsg` values (no bubbletea harness needed): quit keys, cursor
      clamping, archive toggle, search mode enter/filter/clear, tab section
      cycling, collapse-returns-focus, detail open/close, worker file viewer →
      character sheet → back, workflow drill-in cursor, tmux attach guard,
      rainbow easter egg, dataMsg cursor clamp, window resize.
      Package coverage: 28.3% → 41.6%.
- [x] Coverage gate met: package now at **54.1%** (resize-reflow work added
      more tests after the items above).

### Atomic state writes — closed the one bypass

- [x] Exported `state.Create` (locked, atomic temp+rename) and routed
      `workspace.writeStateYAML` through it with the canonical `state.State` struct.
- [x] (found during the fix) The hand-rolled marshal in `work.go` had drifted: the
      owner→worker rename (8ddf6c5) missed it, so scaffolds wrote `stage.owner` /
      `history[].owner` that `state.Load` silently dropped. Round-trip now covered
      by tests in both `state` and `workspace`.
- [x] (same root cause) The embedded templates still instructed agents to run
      obsolete advance/wait-style commands that no longer exist. Fixed to
      `next --worker` / `pause` across stage files,
      ORC.md, AGENTS.md sample worker, and feature README wording.

### `orc adhoc` — already implemented as `orc jit`

No work needed: `orc jit <ticket> --worker <id> "<instruction>"` implements every
decision in the adhoc spec (archive-aware lookup, no stage/status change, per-run
output dir, history entry on completion) plus refinements the spec lacked
(`runtime.jit` visibility in status/TUI, `orc mark <ticket> jit` to close,
concurrent-task blocking). The spec memory predated jit's implementation.

### `handleKey` per-view split (done 2026-06-11)

- [x] The 354-line `handleKey` is now a 15-line dispatcher over five per-view
      handlers (`handleDashboardKey`, `handleDetailKey`,
      `handleWorkflowDetailKey`, `handleFileKey`, `handleCharacterSheetKey`).
      Repetition extracted along the way: `toggleSection` (was four copies),
      `cycleSectionFocus`/`focusSection` (tab cycling), `openViewer` (was four
      copies of the file-viewer setup), `openSectionItem`. Behavior-preserving
      — the full `update_test.go` suite passes unchanged; coverage 54.1% →
      55.5%. The left/right stage-navigation roadmap item now lands in small
      focused handlers instead of a monolith.

### Release automation (done 2026-06-11)

- [x] Tags + releases: `make build` stamps the version from `git describe
      --tags` (was hardcoded `dev`); `.goreleaser.yaml` builds darwin/linux ×
      amd64/arm64 archives; `.github/workflows/release.yml` tests and
      publishes a GitHub release on any `v*` tag push. First tag: `v0.1.0`.

### CI (done 2026-06-11)

- [x] `.github/workflows/ci.yml` runs lint + test on push to main and on PRs.
      golangci-lint pinned to v2.12.2 (matching local), Go version from
      `go.mod`. Same gate as the pre-commit hook, no longer opt-in.

### Pre-commit hook fmt check (done 2026-06-11)

- [x] The gofmt check used `git diff --quiet` over the whole tree, so any
      unstaged file (docs, scratch notes) blocked a commit even when
      formatting was clean. Now keys off `gofmt -l` output directly — fails
      only when Go files actually need reformatting, and names them.

### `orc doctor --fix` (done 2026-06-11)

- [x] `--fix` removes provably-stale state locks — dead PID, or old without a
      valid PID, the same staleness definition the write path's auto-recovery
      uses (`state.ClearStaleLock`, exported from the existing
      `removeStaleLock` logic). Live or ambiguous locks are reported, never
      touched. Covered by tests in `state`, `doctor`, and `cmd/orc`;
      README updated.
- [x] (follow-up) `orc doctor <ticket> --fix` was silently a no-op — the flag
      only applied in workspace mode. Ticket mode now clears that ticket's
      stale lock before validation and prints what it removed.

### Small items

- [x] README dependencies: reworded — chafa is the character-art fallback; pixels
      come natively on kitty/Ghostty via Unicode placeholders.
- [x] `characters1.png` / `characters2.png` moved to
      `internal/tui/assets/portraits/sheets/` (source sheets: hero classes already
      cropped into portraits; orc classes available for future portraits). Outside
      the go:embed pattern, so binary size is unaffected.
- [x] (found during cleanup) Removed stray tracked file
      `internal/tui/assets/portraits/png/.png` — a crop-marked ranger intermediate
      saved with an empty filename, never embedded (go:embed skips dotfiles).
- [x] (found during cleanup) `kittyPortraitSupported` now checks tmux
      `allow-passthrough` and falls back to chafa when it's off — previously the
      placeholder grid would render as garbage because the image transmission was
      silently dropped.
