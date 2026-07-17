# orc plan

Single source of truth for unshipped work. Shipped work belongs in
`CHANGELOG.md`, not here.

## Release candidate

The core v1 feature scope is complete. The remaining release gates are
operational rather than feature work:

- Run the automated, fresh-workspace, and live-session checks in
  [`docs/release.md`](docs/release.md).
- Promote the intended entries from `[Unreleased]`, update `VERSION`, and verify
  the tag-driven release artifacts and checksums.

Direct prompt sending from `orc watch`, notifications, durable arbitrary labels,
remote pack lifecycle, and remote tmux control are explicitly post-v1.

## Later

### Agent completion notification

Add a small notification hook so unattended agent runs can get the user's
attention when they block or complete.

Config shape:

```yaml
settings:
  notify:
    on: [blocked, complete]
    command: "notify-send 'orc' '{{ticket}} {{event}}'"
```

Events:

| Event | When it fires |
|-------|---------------|
| `complete` | Ticket advances to the next stage or finishes |
| `blocked` | `orc mark <ticket> pause` is called |
| `error` | Reserved for future explicit agent failure |
| `all` | Shorthand for all supported events |

Implementation notes:

- Add `NotifySettings` to `internal/config` with `On []string` and
  `Command string`.
- Add `Notify NotifySettings yaml:"notify"` to `Settings`.
- Add `internal/notify`.
- Export `ORC_TICKET`, `ORC_SLUG`, `ORC_EVENT`, `ORC_STAGE`, and
  `ORC_WORKFLOW`.
- Expand `{{ticket}}`, `{{slug}}`, `{{event}}`, `{{stage}}`, and
  `{{workflow}}` as sugar over the same values.
- Run the configured command with a short timeout.
- Fire after state is written in `runMarkNext`.
- Fire in `runMark` for both `pause` and `done`; `orc mark <ticket> done` is
  its own switch arm and must fire `complete`.
- No-op when `command` is empty or the event is not enabled.

Effort: Medium.

## Future ideas

- Explicit, confirmed prompt sending from `orc watch` to an exact agent pane.
- Structured confirm/choice/text human responses.
- Durable arbitrary labels and `--label key=value` filters.
- Remote tmux client selection for standalone watch/focus processes.
- User profiles for personal defaults across workspaces.
- Remote pack installs.
- Pack update and uninstall.
- Pack registries.
- Trust, signing, or provenance beyond local path and digest metadata.
- Per-run log capture for post-mortem debugging of unattended runs.
- Per-worker cost attribution built on `orc report`.
- Homebrew tap, once binary releases have settled.

## Not now

- No guided `orc setup` command; setup remains agent-driven through `SETUP.md`,
  consistent with Orc's agent-first operating model.
- No hosted Orc service.
- No required public pack publishing.
- No generated workspace dependency on a central registry.
- No broad project-management features until the agent workflow loop is tighter.
