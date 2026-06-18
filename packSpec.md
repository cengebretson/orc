# Pack System Spec

## Goal

Let Orc workspaces install multiple reusable packs, then compose workflows from
the stages and workers those packs provide.

Packs should make setup easier without making behavior depend on install order or
silent overwrites.

## Core Model

Packs are reusable libraries of workflow pieces.

A pack may provide:

- named workflows
- stage instruction files
- worker definitions
- optional aliases for friendlier local names

The workspace owns the final `orc.yaml`. A user can run a workflow supplied by a
pack, or create a custom workflow that mixes stages and workers from several
packs.

Guiding rule:

> Packs install reusable workflow pieces. Workspaces compose those pieces into
> the workflows they actually run.

## Namespaced Resource IDs

Pack-provided resources should use stable namespaced IDs:

```text
<pack>:<resource>
```

Examples:

```text
default:bob
default:develop
hotfix:bob
planning:intake
security:reviewer
```

This avoids collisions when multiple packs provide similarly named workers or
stages.

Resource ID grammar:

```text
<pack>:<resource>
```

For the first implementation, both `<pack>` and `<resource>` must use:

```text
lowercase letters, numbers, and hyphens only
```

Examples:

```text
default
hotfix
security-review
qa-automation
```

Disallowed:

```text
Default
hot fix
security_review
team/review
qa:automation
```

This keeps aliases, filesystem paths, and CLI arguments predictable.

Runtime file materialization uses namespace folders:

```text
<pack>:<stage>  -> stages/<pack>/<stage>.md
<pack>:<worker> -> workers/<pack>/<worker>.md
```

Examples:

```text
default:develop -> stages/default/develop.md
hotfix:develop  -> stages/hotfix/develop.md
default:bob     -> workers/default/bob.md
```

### Workers

A worker file from a pack declares a namespaced `id`.

Example:

```yaml
---
id: default:bob
name: Bob the Developer
engine: codex
model: gpt-5-codex
args:
  effort: medium
---
```

### Stages

Stage docs also have stable namespaced IDs, but the IDs live in `pack.yaml`.
Stage markdown stays plain instruction text with no required frontmatter.

Recommended pack manifest shape:

```yaml
stages:
  - id: default:develop
    path: stages/develop.md
```

This allows the installed file to use an ergonomic filename while Orc still
tracks the canonical stage ID.

### Workflows

Pack-provided workflows use namespaced workflow names unless the pack is the
canonical built-in default.

Examples:

```yaml
workflows:
  default:
    description: General feature workflow
    stages:
      - name: default:intake
        worker: default:documentor
        advance: auto

  "hotfix:standard":
    description: Fast hotfix workflow
    stages:
      - name: hotfix:intake
        worker: hotfix:triage
        advance: auto
```

Compatibility decision:

- New workspaces use namespaced canonical IDs for workflows, stages, and workers.
- Custom local resources use the same namespace rule even when no pack is
  installed. For example, `workers/custom/chris.md` resolves to `custom:chris`.
- Root-level runtime files such as `workers/chris.md` are not runnable workers.
- Generated starter workspaces may expose friendly aliases such as `default`.

## Aliases

Namespaced IDs are robust, but noisy. Workspaces may define aliases for common
resources. Aliases live at top-level `aliases:` in `orc.yaml`.

Example:

```yaml
aliases:
  workers:
    bob: default:bob
    reviewer: default:zach
  stages:
    intake: default:intake
    develop: default:develop
  workflows:
    default: default:standard
```

Generated starter workspaces can use aliases so the first-run `orc.yaml` stays
friendly:

```yaml
workflows:
  default:
    stages:
      - name: intake
        worker: fred
        advance: auto
      - name: develop
        worker: bob
        advance: auto
```

Orc resolves aliases before checking that stages and workers exist. The same
resolver is used everywhere resource names are accepted or displayed:

- `orc work --workflow`
- `orc next`
- `orc mark --stage`
- `orc mark --worker`
- `orc status`
- `orc doctor`
- `orc tui`

Rules:

- Aliases are workspace-owned.
- Pack installs may suggest aliases.
- Orc must not silently overwrite an existing alias with a different target.
- If two packs suggest the same alias for the same target, it is allowed.
- If two packs suggest the same alias for different targets, init fails with an
  actionable error.
- If stdin is an interactive TTY, Orc may offer to rename or skip conflicting
  aliases.
- Non-interactive init never prompts. It fails unless the user passes explicit
  conflict-resolution flags in a future version.

## Installing Multiple Packs

A workspace may install multiple packs:

```bash
orc init --pack default --pack hotfix --pack planning
```

The result is one workspace containing all provided workflows, workers, and
stages.

Users can then run:

```bash
orc work STORY-123
orc work HOT-42 --workflow hotfix:standard
orc work PLAN-7 --workflow planning:spec-first
```

Or compose their own workflow in `orc.yaml`:

```yaml
workflows:
  my-flow:
    description: Custom workflow composed from several packs
    stages:
      - name: planning:intake
        worker: planning:analyst
        advance: manual
      - name: default:develop
        worker: default:bob
        advance: auto
      - name: security:review
        worker: security:reviewer
        advance: manual
```

### Default Workflow Selection

The workspace owns `settings.default_workflow`.

Rules:

- If init installs exactly one workflow, Orc may set `settings.default_workflow`
  to that workflow.
- If init installs multiple workflows, the user must choose the default workflow
  explicitly.
- Pack order must not decide the default workflow.
- Suggested workflow aliases may make names friendlier, but they do not choose
  the workspace default when multiple workflows are installed.

Proposed flag:

```bash
orc init --pack default --pack hotfix --default-workflow default
```

If multiple workflows are installed and no default is provided, init fails with
an actionable error:

```text
multiple workflows installed; choose one with --default-workflow
```

## Conflict Rules

Pack installation must be deterministic. No last-one-wins behavior.

Comparison rules:

- Markdown files are compared by exact bytes.
- YAML resources are compared after parsing and re-encoding into Orc's canonical
  YAML form.
- If canonical comparison is not implemented for a resource type, use exact
  bytes and fail on any difference.

### Workflow Conflicts

Two packs cannot provide the same workflow ID unless the definitions are
identical after canonical YAML comparison.

If different, fail:

```text
workflow "hotfix:standard" provided by both hotfix and acme-hotfix
```

### Worker Conflicts

Two packs can provide the same worker ID only if the worker definition is
identical by exact byte comparison.

If different, fail:

```text
worker "default:bob" differs between default and acme
```

### Stage Conflicts

Two packs can provide the same stage ID only if the stage document is
byte-identical.

If different, fail:

```text
stage "default:develop" differs between default and acme
```

### File Path Conflicts

Two packs can write the same file path only if the content is byte-identical.

If different, fail:

```text
stages/develop.md would be written by both default and acme with different content
```

### Alias Conflicts

Two packs can suggest the same alias only if the target is identical.

If different, fail:

```text
alias worker "bob" points to both default:bob and acme:bob
```

## Pack Manifest

Proposed `pack.yaml`:

```yaml
schema: 1
name: default
description: General feature workflow
engines:
  - claude
  - codex

provides:
  workflows:
    - id: default:standard
      path: workflow.yaml
      description: General feature workflow
  workers:
    - id: default:bob
      path: workers/bob.md
      description: Implementation worker
    - id: default:zach
      path: workers/zach.md
      description: Review worker
  stages:
    - id: default:intake
      path: stages/intake.md
      description: Gather ticket context
    - id: default:develop
      path: stages/develop.md
      description: Implement the change

aliases:
  workflows:
    default: default:standard
  workers:
    bob: default:bob
    reviewer: default:zach
  stages:
    intake: default:intake
    develop: default:develop
```

## Pack Commands

Orc should expose pack discovery and inspection commands.

### `orc pack list`

Lists packs. Outside a workspace, it shows packs available to install. Inside a
workspace, it also shows installed packs and whether the current `orc.yaml`
references their resources.

Example:

```text
Available packs:

  default   built-in   General feature workflow
  hotfix    built-in   Fast production fix workflow
  planning  built-in   Spec and planning workflow
  acme      local      Acme backend team workflow
```

Workspace example:

```text
Installed packs:

  default   used     workflows: default, my-flow
  hotfix    used     workflows: hotfix:standard
  security  used     workflows: my-flow
  planning  unused   no workflow references planning:* resources

Available packs:

  docs      built-in  Documentation-only workflow
```

Definitions:

- Available: a built-in or user-installed pack source Orc can install.
- Installed: a pack snapshot exists under `packs/<pack>/` in the workspace.
- Used: at least one workflow in `orc.yaml` references a stage or worker from
  that pack namespace.
- Unused: the pack snapshot exists, but no current workflow references its
  resources.

Usage is derived from `orc.yaml`, not from the `packs/` snapshot alone.

Useful flags:

```bash
orc pack list --json
orc pack list --installed
orc pack list --builtin
orc pack list --available
```

### `orc pack show <pack>`

Shows what a pack installs. In a workspace, it also shows whether each installed
resource is currently referenced by any workflow.

Example:

```text
Pack: default
Source: built-in
Description: General feature workflow

Workflows:
  default:standard  General feature workflow

Stages:
  default:intake    stages/intake.md
  default:develop   stages/develop.md
  default:pr-open   stages/pr-open.md

Workers:
  default:bob       Bob the Developer        codex
  default:zach      Zach the Reviewer        claude

Aliases:
  workflow default  -> default:standard
  stage develop     -> default:develop
  worker bob        -> default:bob
```

Workspace example:

```text
Pack: hotfix
Installed: packs/hotfix

Workflows:
  hotfix:standard  used

Stages:
  hotfix:intake    stages/hotfix/intake.md    used by hotfix:standard
  hotfix:develop   stages/hotfix/develop.md   used by hotfix:standard

Workers:
  hotfix:bob       workers/hotfix/bob.md      used by hotfix:standard
```

Useful flags:

```bash
orc pack show default --json
orc pack show hotfix --workspace ~/my-workspace
```

### `orc pack inspect <path>`

Validates and previews a local pack without installing it.

Example:

```bash
orc pack inspect ./packs/backend-team
```

Output should include:

- parsed manifest
- workflows, stages, workers, aliases
- validation errors
- conflicts against a workspace when run with `--workspace`

### Later: `orc pack install`

Remote or cached installs are useful, but should come after local pack support.

Possible shape:

```bash
orc pack install github.com/acme/orc-packs/backend-team
orc pack install ./packs/backend-team
orc pack uninstall acme
```

Installed packs should live outside workspaces, probably under:

```text
~/.config/orc/packs/
```

## TUI Impact

`orc pack inspect <path>` has no TUI impact. It is a standalone parser and
validator for pack sources.

The TUI is affected later, when workspaces can contain namespaced workflows,
stages, workers, and aliases.

Rules:

- TUI data loading should use the same resource resolver as CLI commands.
- Internal state should prefer canonical IDs such as `hotfix:standard`,
  `hotfix:develop`, and `hotfix:bob`.
- Display should prefer friendly aliases when aliases exist and are unambiguous.
- Dense dashboard views may show aliases only.
- Detail views may show both alias and canonical ID.

Example detail display:

```text
Workflow: hotfix (hotfix:standard)
Stage:    develop (hotfix:develop)
Worker:   bob (hotfix:bob)
```

Potential later TUI additions:

- Filter or group tickets by workflow namespace.
- Show installed and unused packs.
- Link a workflow/stage/worker back to the installed pack snapshot.

These are not part of the first implementation slices.

## Implementation Slices

Implement local pack support before remote pack installs. Keep the first slices
small enough to validate the model without taking on the full pack lifecycle.

Decided implementation order:

1. `orc pack inspect <path>`
2. `orc init --pack <path>` for one external pack
3. multiple pack composition
4. workspace-aware `orc pack list`
5. `orc pack show`

### Slice 1: Inspect Local Packs

Target:

```bash
orc pack inspect ./packs/hotfix
```

Scope:

- Load `pack.yaml` from a filesystem path.
- Validate resource IDs.
- Validate referenced workflow, worker, and stage paths exist.
- Print workflows, stages, workers, aliases, and validation errors.
- Support `--json`.

Out of scope:

- Installing packs into a workspace.
- Merging multiple packs.
- Remote pack install.
- Migrating the current built-in default pack or generated workspace templates to
  namespaced resource IDs.

### Slice 2: Init With One External Pack

Target:

```bash
orc init --pack ./packs/hotfix
```

Scope:

- Load one external pack from a filesystem path.
- Copy its resources into the generated workspace.
- Merge its workflow into `orc.yaml`.
- Apply non-conflicting suggested aliases.
- Fail on conflicts with `_base` or generated files unless content is identical.
- Print what the pack contributed during `--dry-run`.

Out of scope:

- Multiple external packs in one init.
- Pack registry.
- Remote pack install.
- Updating already-initialized workspaces.

### Slice 3: Compose Multiple Packs

Target:

```bash
orc init --pack ./packs/default --pack ./packs/hotfix
```

Scope:

- Load pack metadata from embedded packs and filesystem paths.
- Allow multiple packs in one init.
- Detect workflow, worker, stage, alias, and file path conflicts.
- Require deterministic conflict handling. No last-one-wins behavior.
- Print what each pack contributed during `--dry-run`.
- Keep generated workspace files self-contained.

Out of scope for first slice:

- Remote pack install.
- Pack registry.
- Updating already-initialized workspaces.

## Workspace Ownership After Install

Initial pack install copies files into the workspace. After that, those files are
workspace-owned.

Installed packs have two workspace surfaces:

```text
my-workspace/
  orc.yaml
  stages/
  workers/
  packs/
    default/
      pack.yaml
      workflow.yaml
      stages/
      workers/
    hotfix/
      pack.yaml
      workflow.yaml
      stages/
      workers/
```

The `packs/` directory stores the installed pack snapshots. These snapshots are
for inspection, debugging, future diff/update support, and `orc pack show`.

Orc also materializes runtime resources into the normal workspace folders:

```text
stages/
  default/
    intake.md
    develop.md
  hotfix/
    intake.md
    develop.md

workers/
  default/
    bob.md
    zach.md
  hotfix/
    bob.md
```

Runtime commands use the normal workspace surface as the source of truth:

- `orc.yaml`
- `stages/`
- `workers/`

Custom resources can live under a workspace-owned namespace without being
installed from a pack:

```text
workers/custom/chris.md  -> custom:chris
stages/custom/plan.md    -> custom:plan
```

The namespace folder is required. A file such as `workers/chris.md` is treated as
workspace documentation or a template-like file, not as a runnable worker.

The `packs/` snapshots are not executed directly.

Rules:

- Users may edit installed stage docs, worker files, aliases, and `orc.yaml`.
- Users may inspect pack snapshots under `packs/`, but edits there do not affect
  runtime behavior unless the user also updates `stages/`, `workers/`, or
  `orc.yaml`.
- Orc does not attempt to update, reconcile, or reset installed pack files.
- A future `orc pack update` command would need explicit managed-file metadata
  and conflict handling. That is out of scope for the first implementation.
- `orc pack inspect` validates pack sources. `orc doctor` validates the current
  workspace state.

## Compatibility Notes

No legacy migration is required while orc is still pre-adoption.

Rules:

- Built-in packs use namespaced canonical IDs.
- Generated workspaces include aliases so common commands remain readable.
- Runtime resource files must live under a namespace folder:
  `stages/<namespace>/<stage>.md` and `workers/<namespace>/<worker>.md`.
- A namespace does not imply an installed pack. `custom:*` is a valid local
  namespace owned by the workspace.

## Open Questions

- Should installed pack source metadata be recorded in the workspace for audit
  and future updates?
- Should a future managed-pack mode track diffs between `packs/<pack>/` snapshots
  and materialized runtime files?
