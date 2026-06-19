# Orc Install and Customization Next Steps

## Goal

Make Orc easier to install, try, configure, and tailor for specific users or
teams without requiring changes to Orc's source tree.

## Current Assessment

Orc already has the right core model:

- `orc init` creates a complete workspace.
- `SETUP.md` lets an agent gather workspace-specific details.
- `orc doctor` validates local readiness.
- Workflows, stages, workers, routing, and rules live in editable files.

The main gap is that customization is mostly post-scaffold. A user can edit the
generated workspace, but teams cannot easily package, install, share, or reuse
their preferred setup before running `orc init`.

## Priority 1: Print Post-Init Next Steps

After `orc init` finishes, print the exact next commands.

Example:

```text
Workspace created: /path/to/workspace

Next:
  cd /path/to/workspace
  claude "Read SETUP.md and follow the setup instructions"
  # or:
  codex "Read SETUP.md and follow the setup instructions"
  orc doctor
```

For `--dry-run`, print the same block under a "Would run next" heading.

Why:

- The README explains the flow, but users need the reminder at the moment the
  workspace is created.
- This reduces first-run uncertainty without changing the workflow model.

Acceptance criteria:

- `orc init` prints next steps after successful writes.
- `orc init --dry-run` prints equivalent guidance without implying files exist.
- Existing tests cover the new output.

## Priority 2: Add Guided Setup Command

Add `orc setup` as a CLI path for users who prefer direct prompts over asking an
agent to follow `SETUP.md`.

Initial scope:

- Prompt for ticket system details.
- Prompt for source control details.
- Prompt for repos and purposes.
- Prompt for preferred engines/models per stage.
- Update `ROUTER.md`, `TOOLS.md`, `orc.yaml`, and relevant worker files.
- Mark completed sections in `SETUP.md` so `orc doctor` sees the workspace as
  configured.

Why:

- Agent-driven setup is powerful, but it assumes the user is already comfortable
  delegating setup to Claude or Codex.
- A CLI setup path makes Orc usable even when the user wants explicit control.
- The existing `SETUP.md` can remain the source guide; `orc setup` can implement
  the same questions.

Acceptance criteria:

- `orc setup` is safe to rerun and preserves completed answers by default.
- `orc setup --dry-run` prints planned changes.
- `orc setup --force` can rewrite generated setup fields.
- `orc doctor` passes after a complete guided setup.

## Priority 3: Support External Packs

Make packs installable or selectable from outside the embedded templates.

Proposed commands:

```bash
orc pack list
orc pack install ./packs/backend-team
orc pack install default
orc pack show backend-team
orc init --skip-default-pack
```

Pack shape:

```text
pack.yaml
workflow.yaml
stages/
  intake.md
  develop.md
workers/
  developer.md
  reviewer.md
```

Why:

- Packs are the natural distribution unit for team-specific workflows.
- Today, built-in packs are embedded in the binary, so sharing a custom pack
  effectively means changing Orc itself.
- External packs let teams publish "how we work" without forking the tool.

Acceptance criteria:

- `orc init` installs the built-in default pack unless `--skip-default-pack` is
  passed.
- `orc init --pack ./path` scaffolds a workspace from one local pack.
- `orc pack install ./path` installs a local pack into an existing workspace.
- `orc pack install default` installs a built-in pack into an existing
  base-only workspace.
- `orc pack list` and `orc pack show <pack>` show installed snapshots,
  provenance, and active workflows.
- Pack validation catches missing `pack.yaml`, missing workflow files, duplicate
  worker IDs, and unknown stage references.
- Installed packs refuse canonical ID conflicts, alias conflicts, already
  installed packs, and runtime file overwrites.
- Remote pack installs, update, and uninstall are deferred until their safety
  rules are clear.

## Priority 4: Add User Profiles

Add optional profiles for repeated personal defaults.

Example:

```text
~/.config/orc/profiles/default.yaml
~/.config/orc/profiles/work.yaml
```

Example profile:

```yaml
engine_defaults:
  claude:
    model: claude-sonnet-4-6
  codex:
    model: gpt-5-codex

settings:
  auto_tmux: true
  auto_next: false
  theme: catppuccin-mocha
  tui_refresh: 60

portrait:
  mode: kitty

setup_defaults:
  ticket_system: GitHub Issues
  source_control: GitHub
```

Proposed usage:

```bash
orc init --profile work
orc setup --profile work
```

Why:

- Profiles let individual users carry preferences across workspaces.
- They separate personal preferences from team packs.
- Teams can still share packs, while users keep local model/tool preferences.

Acceptance criteria:

- Profiles are optional.
- `orc init --profile <name>` applies profile defaults before prompts.
- Prompted answers override profile values.
- Workspace files contain resolved values, not references to local profile files.
- Missing profile names produce a clear error.

## Priority 5: Improve Install Distribution

Add lower-friction install paths for non-Go users.

Recommended order:

1. Homebrew tap.
2. Checksums for release binaries.
3. Shell completions.
4. `orc doctor --system` for install-level checks outside a workspace.

Why:

- `go install` is fine for Go developers, but release binaries and Homebrew are
  easier for users who only want the CLI.
- `orc doctor` currently assumes workspace context. A system-level check would
  help users validate the install before creating a workspace.

Acceptance criteria:

- README includes Homebrew install instructions.
- Releases publish checksums.
- `orc completion` or documented Cobra completion output is available.
- `orc doctor --system` checks Orc version, optional tools, PATH visibility,
  tmux, chafa, and supported agent CLIs.

## Suggested Implementation Order

1. Add post-init next-step output.
2. Add external local pack support with `orc init --pack ./path`.
3. Add pack validation and installed pack registry.
4. Add local and built-in `orc pack install/list/show`.
5. Add `orc setup`.
6. Add profiles.
7. Add Homebrew/release distribution improvements.

This order keeps the first improvement small, then expands the customization
model before adding convenience layers.

## Non-Goals

- Do not make generated workspaces depend on a central Orc service.
- Do not require users to publish packs publicly.
- Do not move team-specific conventions into Orc code.
- Do not make profiles required for basic usage.
- Do not replace `SETUP.md`; keep it as the readable setup contract even after
  adding `orc setup`.
