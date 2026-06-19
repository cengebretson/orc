# orc plan

Single source of truth for unshipped work. Shipped work belongs in
`CHANGELOG.md`, not here.

## Up next

### Release and install polish

Make Orc easier to install and validate outside a development checkout.

Remaining:

- `orc doctor --system` for install-level checks outside a workspace.

Acceptance criteria:

- `orc doctor --system` checks Orc version, PATH visibility, optional tools,
  tmux, chafa, and supported agent CLIs.

## Near term

### Pack documentation pass

The local pack system exists. The remaining work is user-facing clarity.

Cover:

- What a pack owns: workflows, stages, workers, and aliases.
- What the workspace owns: final `orc.yaml`, local edits, runtime state, and
  feature files.
- Embedded pack vs installed pack vs local path pack.
- How `orc pack inspect`, `install`, `list`, and `show` fit together.
- What users can safely edit after scaffolding.

## Cleanup

### Multi-pack conflict coverage

Add test coverage once a second embedded pack exists.

Open cases:

- Duplicate workflow name.
- Duplicate worker ID.
- Divergent same-name stage file.
- Duplicate aliases that target different canonical resources.

The first implementation should keep rejecting invisible precedence and require
explicit resolution instead.

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

## Last

### Agent completion notification

Add a small notification hook so unattended agent runs can get the user's
attention when they block or complete. Save this until install/release polish
and pack documentation are done.

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

## Not now

- No hosted Orc service.
- No required public pack publishing.
- No generated workspace dependency on a central registry.
- No broad project-management features until the agent workflow loop is tighter.
