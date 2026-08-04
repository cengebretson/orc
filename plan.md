# Orc roadmap

This file is the source of truth for unshipped work. Shipped behavior belongs
in `CHANGELOG.md`; user-facing behavior belongs in `README.md` and `docs/`.

## Shipped baseline

Orc already provides the durable workflow engine, feature state, worktree
ownership, worker and pack resolution, stage transitions, reporting, session
inventory/resume, watch/dashboard views, notification routing, and manual
session parking.

The live runtime is backend-neutral. tmux remains the portable default, while
the Herdr backend supports exact opaque targets, native agent lifecycle,
worktree create/open, task-cell layouts, attach/focus, sidebar metadata,
notifications, archive cleanup, and structured `orc ctl` state, prompt, wait,
watch, status, and terminal capture. Live views also support opt-in reversible
parking without stopping sessions or changing worktrees.

The tmux-first Super Orc plan is shipped: exact agent identity, provider hooks,
structured lifecycle control, safe prompting, the managed attention rail,
conservative fallback detection, and runtime reconciliation now share the same
backend-neutral inventory.

Release `v0.16.1` successfully published macOS and Linux archives plus
checksums. See `CHANGELOG.md` for the complete shipped history.

## Missing parts

Work in this order:

1. Ship Orc as an additive Herdr plugin.
2. Harden release validation so GoReleaser template failures are caught before
   a tag is pushed.
3. Add `CONTEXT.md`, the project glossary.

## 1. Ship Orc as a Herdr plugin

Package Orc as a thin Herdr UI adapter. The plugin must remain additive: Orc
cannot require Herdr, and Herdr cannot become the owner of workflow state.

Deliverables:

- Add a plugin package under `packaging/herdr/` with a validated
  `herdr-plugin.toml` manifest.
- Expose `orc watch` as a native Herdr pane.
- Expose focused actions for `orc next`, `orc mark next`, and `orc archive`.
- Add a small, readable `orc-herdr-action` shim that parses
  `HERDR_PLUGIN_CONTEXT_JSON`, resolves an exact ticket, and invokes existing
  Orc commands without shell interpolation.
- Use `HERDR_BIN_PATH` for callbacks into Herdr; never assume a bare `herdr`
  executable selects the same server.
- Keep `STATE.yaml` authoritative. `HERDR_PLUGIN_STATE_DIR` may hold only
  Herdr-local presentation preferences.
- Pin the oldest supported Herdr version after validating the manifest against
  the installed Herdr syntax.

Acceptance:

- The plugin links cleanly and opens the Live rail without a dedicated user
  terminal.
- Actions resolve the ticket from exact plugin context and do not act on the
  currently focused human view by accident.
- A normal Orc workspace remains fully usable when Herdr is absent.

## 2. Harden release validation

The `v0.16.0` tag reached GitHub before GoReleaser rejected the build-date
template. `v0.16.1` fixed the template and released successfully, but the same
class of error should be caught before another tag is published.

Deliverables:

- Run the same supported GoReleaser major/minor used by GitHub Actions in a
  non-publishing snapshot validation.
- Make the check evaluate archive names, linker templates, release notes, and
  all platform builds rather than merely parsing YAML.
- Add it to `make release-check` and document it in `docs/release.md` before the
  tag-push step.
- Keep publishing tag-driven and require `VERSION` plus the changelog section
  to match the tag.

Acceptance:

- A broken GoReleaser template fails locally and in CI before a tag is created.
- Snapshot validation cannot create or modify a GitHub release.

## 3. Add the project glossary

Create `CONTEXT.md` with concise, implementation-independent definitions for
stage, loop stage, worker, pack, feature folder, runtime, jit, park, attention,
and other domain terms.

- State that terminology changes must update the glossary.
- Add short ADRs under `docs/adr/` only for decisions that repeatedly get
  reopened.
- Keep the glossary to definitions and durable decisions, not an implementation
  guide.

## Future backlog

- Native Herdr event-stream optimization, named sessions, and remote attach.
- Explicit, confirmed prompt sending from `orc watch` to an exact agent pane.
- Structured confirm/choice/text human responses.
- Explicit remote tmux-client selection for standalone watch/focus processes.
- Durable arbitrary labels and `--label key=value` filters.
- User profiles for personal defaults across workspaces.
- Remote pack install, update, uninstall, registries, signing, and provenance.
- Per-run post-mortem logs and per-worker cost attribution.
- Homebrew distribution.
- Optional external-tracker status mapping with write-back disabled by default.
- Terminal-palette-derived dashboard and watch colors.

## Explicitly out of scope

- A hosted Orc service or required central registry.
- Broad project-management features.
- Terminal compositing or a grid of live drivable panes; Herdr owns that layer.
- Replacing tmux as the default substrate. Herdr remains an optional, richer
  backend while tmux preserves local and server portability.
