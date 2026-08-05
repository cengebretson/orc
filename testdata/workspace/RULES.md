# RULES.md

This file owns permission and approval policy. State transitions belong in
`ORC.md`, routing in `orc.yaml`, and tool selection in `TOOLS.md`.

Workflow, stage, worker, prompt, and operator instructions describe desired
outcomes; they do not grant permission. Workspace Exceptions below may
explicitly authorize automation that would otherwise require approval.

## Ask Before Acting

Pause the ticket with the exact proposed action before asking for approval to:

- Delete files, branches, worktrees, or feature context.
- Rewrite Git history, force-push, reset, or amend published commits.
- Push to a remote, open or merge a PR, post comments, change tickets, trigger
  CI/CD, or otherwise modify an external system.
- Change dependency manifests or lockfiles, or install global/system software.
- Modify production, staging, shared infrastructure, permissions, or billing.
- Read, copy, expose, or change secrets, credentials, tokens, private config, or
  environment files.
- Start a persistent service or background agent that will outlive the session.
- Run an unusually costly operation or a command expected to take over 10
  minutes.
- Change shared workspace contracts, templates, setup scripts, or approval
  policy outside the explicit task.

Approval applies only to the action and scope presented. A prior approval does
not authorize a broader follow-up action.

## Allowed Without Additional Approval

Within the active ticket and stage, agents may:

- Read and search workspace and repository files.
- Create or edit ticket artifacts and implementation files.
- Create a ticket branch and worktree at the configured workspace path.
- Run documented worktree setup, including installing already-declared project
  dependencies inside the isolated worktree.
- Run targeted local formatting, linting, builds, and tests.
- Inspect local Git state and create local commits when the stage requires it.
- Draft plans, summaries, patches, PR text, and proposed external updates.
- Use `orc mark` and edit only the agent-writable `STATE.yaml` context fields
  listed in `ORC.md`.

These permissions do not override repository-specific instructions or a more
restrictive current-stage rule.

## Workspace Exceptions

Setup should record confirmed deviations from the defaults here. Write explicit
actions and scope; do not use vague grants such as “full access.”

- <!-- e.g. Agents may push ticket branches to org/repo after local checks. -->

## When Unclear

Ask when an action could materially surprise the human, affect another person,
or change shared or external state. For ticket work, first run
`orc mark <ticket> pause "<decision or approval needed>"` so state reflects who
must act next.
