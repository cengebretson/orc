# Workspace Setup

This file is an execution playbook for the agent configuring this workspace.
Perform the setup yourself: inspect the local environment, propose sensible
defaults, make the workspace edits, and verify the result. Do not turn this file
into a checklist for the user to execute manually.

## Status

shared: pending
claude: pending
codex:  pending

<!-- orc doctor checks these lines — do not remove them. -->
<!-- Change each to "complete" when that section is finished. -->

## Operating Rules

- Resume from the Status block. Do not redo completed sections.
- Inspect before asking. Read the generated files, installed packs, configured
  tools, and relevant repository instructions first.
- Infer facts that can be verified locally. Ask the user only for preferences,
  credentials they must provide, or choices with materially different outcomes.
- Never invent a ticket retrieval command, repository path, approval policy, or
  external integration name. Propose a verified default when possible and ask
  for confirmation.
- Ask remaining questions in one concise message. Explain any proposed defaults
  so the user can accept or correct them together.
- Make all file edits yourself. Do not ask the user to copy snippets or run setup
  commands unless authentication or another interactive boundary requires it.
- Keep each fact in its owning file; do not duplicate workspace policy.

## 1. Discover the Workspace

Before asking questions:

1. Read `orc.yaml`, `TOOLS.md`, `RULES.md`, and `AGENTS.md`.
2. Run `orc pack list` and inspect the active workflow and worker assignments.
3. Inspect each proposed repository for its remote, default branch, nearest
   agent instructions, README, and build/test entrypoints.
4. Inspect the available Claude and Codex configuration and tools without
   exposing secrets. Record integration names only, never tokens or values.
5. Run `orc doctor` once. Use its findings as part of the setup checklist.

Summarize what was discovered and distinguish verified facts from proposed
defaults.

## 2. Confirm the Undiscoverable Decisions

Ask one consolidated set of questions, omitting anything already known:

- Which ticket system should this workspace use, and what exact command or tool
  retrieves a ticket by ID? If a likely command is available, propose it for
  confirmation. Ask for project keys, labels, or components only when relevant.
- Which discovered repositories belong in this workspace, and is any expected
  repository missing? Confirm purpose and routing when it is not obvious.
- Should the installed workflow remain as-is? Show its stage sequence briefly
  and ask only about desired changes, required handoff artifacts, and whether
  missing artifacts should `warn` or `block`.
- Which agent engine should run each worker role? Prefer existing assignments.
  Ask whether each engine should use its configured default model or an explicit
  model; do not present a hard-coded model catalog.
- Do the approval defaults in `RULES.md` fit the team? Ask only about exceptions.
- Are there team conventions that agents cannot learn from repository files?

If the user accepts the proposed defaults, proceed without another interview.

## 3. Configure Shared Workspace Files

Apply the confirmed setup with these ownership boundaries:

| File | Owns |
|---|---|
| `ORC.md` | Orc's durable session and state contract; keep the generated protocol unless intentionally upgrading it |
| `orc.yaml` | repositories, purposes, ticket-metadata routing, agent hints, worktree setup, workflows, worker routing, artifact policy |
| `TOOLS.md` | ticket retrieval, source-control identity, access method, and integration names |
| `RULES.md` | approval requirements and external-write policy |
| `AGENTS.md` | team conventions that apply across repositories |

Repository rules:

- Write discovered, canonical absolute repository paths in `orc.yaml`. Do not
  preserve the generated path placeholder or assume repos are workspace siblings.
- Prefer `projects/<repo-name>/` for repositories cloned during setup. Reuse an
  existing external checkout in place rather than moving or duplicating it.
- Add short `agent_hints` for stable repo entrypoints and verification habits.
- Add `routing` rules only for reliable ticket labels or components. A rule may
  select multiple repositories explicitly. Do not treat a ticket ID prefix as
  repository ownership, and do not couple repository routes to workflows. Make
  rules mutually exclusive; intentional cross-repo work belongs in one rule.
- Add `worktree_setup` only when raw `git worktree add` is insufficient. It must
  include `{{worktree_path}}`; supported placeholders are `{{branch}}`,
  `{{worktree_path}}`, `{{repo_path}}`, `{{repo_name}}`, `{{ticket}}`, `{{slug}}`,
  and `{{workspace}}`.
- Keep stage-specific instructions in stage or worker files, not `agent_hints`.

Workflow rules:

- Keep routing in `orc.yaml`; worker files describe how a role behaves.
- Every referenced worker must resolve to `workers/<namespace>/<name>.md` with a
  matching `<namespace>:<name>` frontmatter ID.
- Edit workers installed by a pack in place. When no pack supplies a worker,
  copy `workers/_template.md` into a namespaced path.
- Configure only artifacts that make a stage handoff repeatable. Avoid requiring
  files merely because the template contains them.

After shared files are correct, change `shared: pending` to `shared: complete`.

## 4. Configure Agent Engines

For every worker used by the active workflow:

- Set exactly one supported `engine` (`claude` or `codex`).
- Set `model` only when the user chose an explicit model; otherwise preserve the
  engine's configured default.
- Record available MCP server or tool names in `TOOLS.md`. User-level integration
  configuration remains outside this workspace.

For an engine the user does not want, make no worker assignments to it and mark
its status complete. For an enabled engine, verify its command is available and
its assigned workers are valid before marking that engine complete.

Change the corresponding `claude:` or `codex:` status line to `complete` after
that engine has been handled.

## 5. Verify and Hand Off

1. Run `orc doctor` and fix every setup-related failure. Explain warnings that
   are intentional; do not silently declare them healthy.
2. Run `orc pack list` and confirm the active workflow has no unresolved stages
   or workers.
3. If feature folders already exist, run `orc artifacts <ticket>` for one active
   ticket to verify the configured handoff contract.
4. Review the diff for placeholders, duplicated facts, secrets, and accidental
   edits outside the generated workspace files.
5. Report the chosen ticket route, repositories, workflow, worker-to-engine
   assignments, approval exceptions, files changed, and verification result.

When at least one engine has runnable workers and `orc doctor` is clean, setup is
complete. Tell the user the single next command: `orc work <ticket>`.
