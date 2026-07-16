# orc plan

Single source of truth for unshipped work. Shipped work belongs in
`CHANGELOG.md`, not here.

## Up next

### Tmux popup workflow

Document optional, user-owned tmux bindings for Orc's completed live-session
commands:

- Toggle the compact `orc watch` rail.
- Open `orc sessions --all` in a popup.
- Open the interactive `orc sessions resume` picker in a popup.
- Jump directly to `orc focus`.

Keep these as copyable configuration examples. Orc must not install or mutate
tmux configuration automatically.

Acceptance: the documented bindings use current command flags, cover popup and
split-pane variants where appropriate, explain workspace selection, and pass a
manual `tmux source-file` syntax check.

Effort: Small.

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

- `orc setup`: optional prompt-driven setup for users who do not want to ask an
  agent to follow `SETUP.md`.
- User profiles for personal defaults across workspaces.
- Remote pack installs.
- Pack update and uninstall.
- Pack registries.
- Trust, signing, or provenance beyond local path and digest metadata.
- Per-run log capture for post-mortem debugging of unattended runs.
- Per-worker cost attribution built on `orc report`.
- Homebrew tap, once binary releases have settled.

## Not now

- No hosted Orc service.
- No required public pack publishing.
- No generated workspace dependency on a central registry.
- No broad project-management features until the agent workflow loop is tighter.
