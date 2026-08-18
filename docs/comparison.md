# How Orc compares

Orc is the durable workflow layer for coding agents. It keeps plans, decisions,
state, and handoffs alive across agents and sessions. Tmux and Herdr are
execution backends, not the product boundary.

The projects below are useful reference points, but they optimize for different
parts of the agent-development stack.

| Project | Center of gravity | Relationship to Orc |
| --- | --- | --- |
| [Paseo](https://paseo.sh/) | Cross-device agent platform with desktop, web, mobile, and CLI clients | Paseo optimizes for reaching and controlling agents from anywhere. Orc optimizes for durable workflow state and explicit handoffs. |
| [jmux](https://jmux.build/) | Ticket-to-merge workflow and fleet cockpit layered over tmux | Jmux is the closest overlap. It has deeper tracker, forge, CI, diff, and fleet UI integrations; Orc keeps the workflow itself portable and auditable in files. |
| [tmux-ide](https://www.tmux-ide.com/) | Agent-aware chrome, status, notifications, restore, and IDE surfaces added to existing tmux sessions | tmux-ide is primarily an operational control plane. Orc is primarily a workflow and handoff engine. |
| [Herdr](https://herdr.dev/) | Agent-aware terminal multiplexer with persistent PTYs, remote attach, automation APIs, and plugins | Orc supports Herdr as an optional backend while retaining ownership of durable workflow state. |

## What Orc optimizes for

- **Durable context:** each feature folder records the ticket, plan, decisions,
  workflow state, stage outputs, and history.
- **Explicit handoffs:** stages define inputs, outputs, exit criteria, and
  required artifacts.
- **File-owned policy:** workflows, workers, model choices, repository setup,
  and review gates stay in YAML and Markdown rather than command handlers.
- **Agent portability:** Claude, Codex, and other command-line agents can share
  the same workflow without an Orc-specific SDK.
- **Recoverable execution:** durable state remains authoritative when a process,
  terminal, tmux server, or agent session disappears.

## What Orc does not try to replace

Orc is not a terminal emulator, hosted control plane, mobile client, source-code
editor, diff viewer, issue tracker, or pull-request UI. Those tools can remain
the best tool for their job while Orc carries work between them.

New integrations should preserve that boundary. Prefer small adapters, commands,
and structured output over rebuilding a terminal, tracker, or IDE inside Orc.

## The durable boundary

Live telemetry and multiplexer metadata improve the operator experience, but
they are overlays. `STATE.yaml` and the feature-folder artifacts remain the
durable source of truth. When live and durable state disagree, Orc preserves the
durable record and reports the runtime discrepancy rather than silently
rewriting history.
