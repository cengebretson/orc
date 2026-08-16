# Orc context

This glossary defines Orc's durable project vocabulary. It is for contributors
working on Orc itself; it is not copied into generated workspaces. Keep code,
commands, UI labels, specifications, and documentation aligned with these
definitions. A terminology change must update this file in the same change.

## Workspace and work

**Workspace** — The filesystem root created and managed by Orc. It contains
configuration, workflow policy, feature state, repository registrations, and
optional runtime integrations. A workspace is durable and does not depend on a
running terminal multiplexer.

**Ticket** — The external or local identifier used to address a unit of work,
such as `ORC-123`. Orc does not require an external tracker to own the ticket.

**Feature** — Orc's durable representation of one ticket inside a workspace.
A feature carries workflow state, handoff context, artifacts, and runtime
references across agent sessions.

**Feature folder** — The directory under `features/` that stores a feature's
`STATE.yaml` and durable handoff material. Conversation history and terminal
scrollback are not substitutes for this folder.

**Repository** — A source checkout registered in `orc.yaml`. One feature may
select one or more repositories, each with its own worktree and commands.

**Worktree** — A repository checkout assigned to feature work. It is distinct
from the feature folder: the worktree holds source code; the feature folder
holds Orc state and handoff context.

## Workflow

**Workflow** — An ordered policy describing how a feature moves through
stages. Workflow configuration selects stages and transition behavior; it does
not contain the feature's current state.

**Stage** — A named step in a workflow, such as planning, implementation, or
review. The stage file defines the current task contract, exit criteria, and
required outputs.

**Loop stage** — A stage that may repeat through configured workers or passes
until its loop policy is satisfied. A loop stage is still one workflow stage;
its iterations are not separate stages.

**Worker** — A named agent role and launch policy. A worker selects an engine,
model or cost preferences, instructions, and capabilities. It is not a running
agent process.

**Pack** — A reusable bundle of workflow policy and supporting files that can
be resolved into a workspace. A pack supplies defaults; workspace-owned files
remain the effective local policy.

**JIT task** — A one-off agent task outside the configured stage pipeline, such
as a spot check or secondary review. It does not change the feature's stage or
status; its live record is `runtime.jit`, and its outputs are stored under the
feature folder's `jit/` directory.

**Artifact** — A stage-required durable output whose presence and change state
Orc may validate before transition. Terminal output alone is not an artifact.

## Durable and live state

**Durable status** — The workflow state persisted in `STATE.yaml`, such as
pending, active, paused, ready, or done. It survives process, terminal, and
machine-session boundaries.

**Runtime** — The persisted description of where and how active feature work
is running. Runtime data points to a backend target and may include agent and
provider identities; it is evidence about execution, not workflow authority.

**Backend** — An implementation of Orc's terminal/runtime boundary, currently
tmux or Herdr. Backends own terminal composition and exact target operations;
Orc owns workflow state and durable identity.

**Target** — A backend-owned exact workspace/session, tab/window, and pane
location. Target identifiers are opaque and must be validated rather than
reconstructed from display labels.

**Agent identity** — Orc's durable identifier for the logical agent assigned
to feature work. It can survive restoration or provider resumption.

**Agent instance** — The identifier for one live launch of an agent identity.
A restoration creates a new instance even when it resumes the same provider
session.

**Provider session** — A provider-owned resumable conversation identifier. It
is neither Orc's durable agent identity nor proof that a process is currently
live.

**Lifecycle** — Recognized live execution state such as unknown, idle,
working, blocked, or done. Native backend state or installed provider hooks are
authoritative; terminal-title and screen inference is presentation-only.

**Reconciliation** — Orc's comparison of durable runtime identity with current
backend and provider evidence. Results are `live`, `resumable`, `replaced`,
`orphaned`, or `unknown`.

## Presentation and session management

**Attention** — A live signal that work may need human action or review. Its
source matters: authoritative hook/native attention can drive structured
behavior, while title or screen inference can only decorate the UI.

**Rail** — Orc's managed `orc watch` view presented as a normal resizable tmux
pane. The rail displays state; it does not replace tmux's layout or controls.

**Park** — Reversibly hide an eligible live session from configured Live views
without stopping its process or changing its worktree. Parking is presentation
state, not a workflow transition.

**Restore / unpark** — Return parked work to Live presentation. If its old
process is gone but its identity is safely resumable, restoration launches a
new agent instance and waits for authoritative registration before replacing
the recorded runtime.

**Managed session** — A live backend session that Orc can associate with
durable feature state and validate against its recorded runtime identity.

**Orphaned session** — A session or durable runtime whose expected counterpart
cannot be proven. Orc reports it conservatively rather than adopting or
deleting it by label.

## Durable decisions

- `STATE.yaml` is authoritative for workflow state.
- Feature folders are the durable handoff boundary.
- Backend target IDs and agent instance IDs require exact matching.
- Terminal text is diagnostic or presentational, never authoritative lifecycle.
- Sources that may drive actions are named explicitly; anything else renders but
  never acts ([ADR 0004](docs/adr/0004-authoritative-sources-are-named.md)).
- `tmux-attention` is a display layer Orc consumes, never an authority it
  depends on, and neither tool absorbs the other
  ([ADR 0005](docs/adr/0005-orc-and-tmux-attention-are-layered.md)).
- tmux remains the portable default; Herdr is an optional additive backend
  ([ADR 0001](docs/adr/0001-tmux-is-the-default-backend.md)).
- Repeatedly reopened architectural decisions belong in short records under
  [`docs/adr/`](docs/adr/), not in additional glossary prose.
