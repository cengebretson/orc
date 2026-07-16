# orc plan

Single source of truth for unshipped work. Shipped work belongs in
`CHANGELOG.md`, not here.

## Up next

### Context-pressure presentation

Continue the next Recon-inspired session UX slice described in `recon.md`:

- Display provider context usage in `orc watch` and `orc tui`.
- Use configurable green, yellow, and red thresholds.
- Treat missing or unknown provider limits as unavailable rather than zero.
- Keep context pressure presentation-only; it must not mutate workflow state,
  terminate a session, or advance a stage.

Acceptance: focused rendering tests cover threshold boundaries and unknown
limits, configuration validation rejects invalid threshold order, and
`make check` passes.

Effort: Medium.

## Queued

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
