# orc plan

Single source of truth for unshipped work. Shipped work belongs in
`CHANGELOG.md`, not here.

## Up next

### Guided workspace setup

Add an optional `orc setup` path for users who prefer a guided workspace setup
over asking an agent to follow `SETUP.md`.

- Inspect the current `orc.yaml`, repository paths, and installed workflow packs.
- Prompt only for missing or invalid setup inputs.
- Preview proposed file changes before writing them.
- Preserve hand-edited workspace policy and never overwrite it silently.

Acceptance: a fresh scaffold can reach a clean config/readiness state through
the guided path, an existing configured workspace is left unchanged, and
non-interactive or cancelled runs do not partially write configuration.

Effort: Medium.

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
