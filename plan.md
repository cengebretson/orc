# Orc roadmap

This file is the source of truth for unshipped work, and holds nothing else.
`CHANGELOG.md` is the shipped history, `README.md` and `docs/` describe current
behavior, `CONTEXT.md` defines the vocabulary, and `docs/adr/` records why the
load-bearing decisions are what they are. Restating any of that here would only
drift out of step with it.

Nothing is currently in progress. Pick from the backlog below, or take up the
deferred Herdr plugin.

## Deferred: Herdr plugin

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

## Future backlog

- Native Herdr event-stream optimization, named sessions, and remote attach.
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
