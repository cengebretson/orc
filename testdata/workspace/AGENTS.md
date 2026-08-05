# AGENTS.md

## Scope

This is the shared workspace entrypoint for Claude, Codex, and other agents.
It explains which workspace contracts apply and holds team conventions that
span repositories. `CLAUDE.md` imports it; Codex reads it directly.

Keep product-specific commands and standards in the owning repository. Put
stage behavior in stage files and worker-specific behavior in worker files.

## Read First

- `ORC.md` owns ticket session protocol, durable state, repository-routing
  preconditions, handoffs, and worktrees.
- `orc.yaml` owns repository definitions and routing data, workflows, stages,
  workers, and workspace settings.
- `RULES.md` owns permission and approval boundaries. It remains authoritative
  when a workflow, stage, worker, or prompt requests an external or permanent
  action.
- `TOOLS.md` owns verified integrations, external access, and preferred tools.
- The current stage file owns the task, exit criteria, and required outputs.
- Repository-local instructions own product commands and coding conventions.

## Ticket Sessions

Follow the complete Session Protocol in `ORC.md`; do not substitute a second
lifecycle procedure from this file. Start by inspecting
`orc status <ticket> --json`, then read the feature context and current stage
before acting. End every active session with the durable transition required by
the stage and `ORC.md`.

The feature folder under `features/<ticket-slug>/` is the durable handoff. Do
not rely on conversation memory for facts that the next stage or agent needs.

## Repository Commands

Use the repository selection persisted in `STATE.yaml.repos`, resolving it by
the `ORC.md` contract when missing or incomplete. A main checkout will commonly
live under `projects/<repo-name>/`, but `orc.yaml` may point to an external
absolute path; always use the configured path.

Run repository-specific commands from the selected repository or ticket
worktree, never from the workspace root unless a workflow explicitly requires
it. Code-changing ticket work uses
`worktrees/<repo-name>/<ticket-slug>/` and must remain recorded in feature state.

---

## Team Conventions

<!-- This section is yours. Add cross-repository conventions, review
     expectations, naming rules, or other stable guidance that cannot live in
     a single repository. Orc does not read or modify this section. -->
