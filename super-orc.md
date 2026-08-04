# Super Orc: tmux-first agent orchestration

Status: draft implementation specification

## Summary

Super Orc makes tmux the primary Orc experience while bringing the valuable
agent-native behavior of Herdr to the tmux backend. Orc owns workflow state,
agent identity, lifecycle events, exact targeting, prompting, waiting,
notifications, restoration, and the Live rail. tmux continues to own terminal
composition, sessions, windows, panes, key bindings, mouse behavior, and user
customization.

Herdr remains an optional richer backend. It must not be required for normal
Orc use, and Orc must not reproduce a user's tmux cosmetics inside Herdr.

## Motivation

Orc already has a strong tmux workflow, a backend-neutral runtime model, and
hook-backed structured lifecycle reads and waits. The remaining control gap is
safe exact-instance prompting and prompt-stall detection; later phases add the
managed Live rail, reconciliation, and conservative presentation fallbacks.

The goal is not to turn tmux itself into Herdr. The goal is to let Orc supply
the missing agent semantics while preserving the tmux environment users have
already customized.

## Product outcome

A user should be able to keep their existing tmux configuration and gain:

- durable Orc agent identity independent of a pane label;
- recognized `idle`, `working`, `blocked`, `done`, and `unknown` lifecycle;
- separate, age-aware human-attention state;
- exact agent state, prompt, wait, watch, and capture commands;
- reliable prompt-stall and process-exit detection;
- an Orc-owned, mouse-resizable Live rail in a normal tmux pane;
- agent completion and block notifications;
- safe reconciliation after tmux, pane, or agent restarts;
- conservative screen-derived presentation when hooks are unavailable.

## Design principles

1. **tmux remains tmux.** Orc does not own the user's status line, key table,
   theme, window naming policy, or general pane layout.
2. **Durable state wins.** `STATE.yaml` remains authoritative for workflow
   state. Live agent state may inform presentation and control but cannot
   silently advance a workflow stage.
3. **Exact targets only.** Input, focus, capture, and control must resolve a
   recorded session, window, pane, and agent instance before acting.
4. **Hooks are authoritative; screen text is not.** Supported provider hooks
   may publish lifecycle. Screen detection supplies presentation fallback only.
5. **Lifecycle and attention are separate.** An agent can be idle while its
   unseen completion still needs attention.
6. **No required service.** The design remains local, workspace-friendly, and
   usable over SSH. No hosted registry or daemon is required.
7. **Backend capability is explicit.** tmux implements richer mux interfaces
   only when their safety contracts can be met; callers never infer support
   from the backend name.
8. **Degrade visibly.** Missing hooks produce `unknown` or no live overlay,
   never a confident but invented lifecycle state.

## Existing baseline

The following behavior is already shipped and should be reused:

- backend-neutral `runtime.mux` targets;
- exact tmux session, window, and pane recording;
- validation before focus and capture;
- pane and window metadata for ticket, stage, worker, engine, feature folder,
  and provider session;
- session inventory and resume;
- `orc watch`, aggregate status, session capture, and reversible parking;
- general notification routing;
- the Herdr `mux.AgentControlBackend` implementation as a behavioral reference.

The tmux backend already implements `mux.Backend`, `mux.TargetBackend`, and
`mux.TerminalCaptureBackend`. Super Orc adds reliable agent identity and then
implements `mux.AgentControlBackend` for tmux.

## Architecture

```text
Codex / Claude hooks
        |
        v
  orc agent-event  -----> live agent registry
        |                  | identity + sequence
        |                  | lifecycle + attention
        v                  v
 tmux pane options     STATE.yaml target
        |                  |
        +--------+---------+
                 v
          tmux AgentControlBackend
             |     |     |
           state prompt  wait
                 |
                 v
     orc ctl / orc watch / notifications
```

The first implementation may poll tmux metadata at the existing watch
interval. A tmux control-mode event stream is an optimization, not a launch
requirement.

## Identity model

### Durable identity

Every Orc-launched agent receives an opaque Orc agent ID. It is not derived
from the ticket, worker, stage, window name, pane order, PID, or provider
session ID.

Recommended form:

```text
agent_id = "a_" + random identifier
```

`STATE.yaml` records the agent identity beside the backend-neutral target:

```yaml
runtime:
  mux:
    backend: tmux
    workspace: orc-hot-42
    tab: develop
    pane: "%12"
  agent:
    id: a_01...
    instance: i_01...
    engine: codex
    provider_session: 019...
```

The exact schema may be adjusted to match existing state conventions, but
identity and target must remain separate concepts.

### Live instance identity

Each agent launch receives a new opaque instance ID. Orc stamps it onto the
exact pane and records it durably. Replacing or restarting the agent creates a
new instance even when the pane survives.

Minimum tmux metadata:

```text
@orc_agent=1
@orc_agent_id=<durable ID>
@orc_agent_instance=<launch ID>
@orc_provider_engine=<engine>
@orc_provider_session=<provider ID when known>
@orc_agent_state=<lifecycle>
@orc_agent_state_seq=<monotonic sequence>
@orc_agent_state_since=<unix time>
@orc_agent_state_source=<hook|native|screen>
@orc_agent_attention=<input|blocked|review|done|empty>
@orc_agent_attention_since=<unix time>
@orc_agent_attention_source=<hook|screen>
```

Orc must validate the recorded pane, agent ID, and instance ID before sending
input. Session/window membership alone is insufficient because a live pane can
acquire a different process.

### Restart behavior

- A missing tmux server makes the agent offline, not completed.
- A missing pane makes the recorded instance orphaned.
- A matching pane with a different instance is replaced and cannot receive
  input through the old record.
- A provider session that can be resumed creates a new live instance while
  retaining the durable Orc agent ID.
- tmux restoration never proves that the restored process is the old agent;
  the provider hook must re-register it.

## Lifecycle model

Supported lifecycle values:

| State | Meaning |
|---|---|
| `idle` | Recognized agent is ready for a prompt. |
| `working` | Agent accepted work and has not settled. |
| `blocked` | Agent is waiting for approval or a human response. |
| `done` | Background work settled and has not been seen. |
| `unknown` | Agent exists but Orc cannot classify it confidently. |
| `offline` | Orc record exists but no matching live instance exists. |

`done` is a presentation-aware settled state. Focusing or explicitly
acknowledging the agent changes `done` to `idle` without changing durable
workflow state.

Every authoritative lifecycle change increments `state_seq` and records
`state_since`. Duplicate hook delivery is idempotent and must not increment the
sequence.

### Attention model

Attention is an overlay, not a lifecycle authority:

| Attention | Meaning |
|---|---|
| `input` | A normal human response is requested. |
| `blocked` | Approval or a blocking decision is required. |
| `review` | Work is ready for human review. |
| `done` | Work completed in the background. |
| empty | Nothing currently requests attention. |

The rail displays attention age and a distinct stuck treatment after a
configurable threshold. Hook-derived values always outrank screen-derived
values.

## Agent event ingestion

Orc owns a small, stable command used by provider integrations. The final CLI
name may be hidden from normal help, but its logical contract is:

```text
orc agent-event \
  --engine codex \
  --agent-id a_01... \
  --instance i_01... \
  --state working \
  --provider-session 019... \
  --event-id evt_01...
```

Requirements:

- Resolve the source pane from `TMUX_PANE`; do not accept an arbitrary pane
  from ordinary hook execution.
- Prove the pane carries the matching agent and instance IDs.
- Validate enumerated states and bounded field lengths.
- Deduplicate by event ID or provider event identity.
- Apply metadata updates atomically enough that readers never see a new
  sequence paired with old state.
- Clear stale attention when a newer authoritative event supersedes it.
- Return quickly; hook delivery must not noticeably delay the agent UI.
- Never edit `STATE.yaml` workflow stage or status in response to an event.

An internal workspace-local event journal may be added for reconciliation and
debugging. It must be bounded, redact prompt content by default, and remain
optional for control correctness.

## Hook installer

Add the planned idempotent command surface:

```text
orc doctor --install-agent-hooks
orc doctor --install-agent-hooks --dry-run
```

Each engine integration reports:

- executable presence and detected version;
- configuration path used for this invocation;
- hook support and observable lifecycle states;
- installation state and required user approval;
- whether uninstall can safely remove only Orc-owned configuration.

Safety requirements:

- Preserve unrelated user configuration and formatting where practical.
- Preserve unparseable configuration byte-for-byte and report failure.
- Respect alternate configuration roots such as `CLAUDE_CONFIG_DIR`.
- Mark inserted configuration with an Orc-owned stable identifier.
- Reinstallation is idempotent across Orc upgrades.
- Dry-run reports exact intended files and logical changes without writing.
- Codex hook enablement must not silently approve hook hashes for the user.

### Porting the Herdr hook scripts

Herdr ships working hook integrations for both engines Orc cares about, at
`src/integration/assets/{claude,codex}/herdr-agent-state.sh` in that
repository. They are roughly a hundred lines of Python each, invoked from a
small shell wrapper, and they are worth reading before writing new ones.

Only the transport differs. Herdr's scripts write one JSON-RPC line to a Unix
socket; the tmux equivalent sets a pane or window option that `orc watch` and
the status line already read. Everything above the transport transfers.

What transfers, and is the actual value:

- **Subagent filtering.** A hook payload carrying `agent_id` is a subagent
  event. Reporting it moves the pane's state on behalf of something the user
  is not waiting on.
- **`SubagentStop` is not a resume.** Claude emits it from recap and
  away-summary flows after the main turn has already stopped. Treating it as
  durable working revives an idle pane and produces a rail that claims work is
  happening when none is.
- **Monotonic sequence numbers.** Nanosecond timestamps order concurrent
  reports so a slow hook cannot overwrite a newer state.
- **Session identity.** Session id and transcript path let a report bind to a
  specific agent session rather than only to a pane, which matters across
  restarts and reattachment.
- **Silent failure.** Every script exits zero when the environment is absent.
  A hook that errors inside the user's agent is worse than a missing state.

What Orc must decide that Herdr did not have to:

- Which tmux option namespace carries state, and whether it is pane-scoped or
  window-scoped. Pane scope is more precise; window scope is what existing
  status-line integrations already read.
- Whether to reuse `tmux-attention`'s `@agent_attention` contract directly, or
  define an Orc-owned option and let `tmux-attention` remain a separate
  consumer.

Screen scraping stays a fallback, not a requirement. Both target engines
support hooks, so a first implementation can ship without any pane-content
parsing and add it later only if gaps appear.

## tmux agent control backend

After authoritative events and identity validation exist, tmux implements
`mux.AgentControlBackend`.

### State

`StateAgent`:

1. validates the backend, target, pane membership, agent ID, and instance ID;
2. reads the latest authoritative state and sequence;
3. reports `unknown` when the agent is live but classification is unavailable;
4. reports a stable structured error for stale, replaced, or offline targets.

Screen-derived state may appear in Live presentation but cannot satisfy this
API.

### Prompt

`PromptAgent`:

1. validates the exact live agent instance immediately before input;
2. rejects NUL and unsafe control data and applies a documented size limit;
3. submits text through a uniquely named tmux buffer with bracketed-paste-safe
   behavior, then sends encoded Enter;
4. deletes the temporary buffer on success and failure;
5. never uses a shell command containing interpolated prompt text;
6. records the starting lifecycle sequence;
7. when waiting, requires an observed authoritative change within the startup
   grace period before waiting for a settled state.

If the sequence does not change, return a stable `agent_prompt_stalled` error.
If the pane or instance changes during delivery, return `agent_replaced`.

Prompting from `orc watch` remains an explicitly confirmed action. Background
automation uses `orc ctl` and an exact ticket or agent target.

### Wait

`WaitAgent` waits for lifecycle events rather than terminal text. Default
settled states are `idle`, `done`, and `blocked`; `--until` may request an exact
supported subset.

It must distinguish:

- timeout;
- agent process exit;
- pane replacement;
- backend disappearance;
- unsupported or unknown lifecycle;
- successful settlement.

Polling tmux options is acceptable initially. Waits must be context-cancelable,
bounded when a timeout is supplied, and inexpensive with many agents.

### Watch and aggregate status

Existing `orc ctl agent watch` and `orc ctl status` should automatically include
tmux agents once tmux implements the control capability. Emission remains
change-driven JSONL, keyed by durable Orc agent identity rather than pane ID.

## Live rail in tmux

Add a managed rail command, provisionally:

```text
orc rail open
orc rail close
orc rail toggle
```

Behavior:

- Open `orc watch` in a real side pane in the current tmux window.
- Reuse an existing Orc-owned rail instead of creating duplicates.
- Default to a configurable width near 64 columns.
- Allow normal tmux mouse resizing; never continuously force the configured
  width after creation.
- Preserve focus unless the user explicitly asks to enter the rail.
- Stamp ownership and role metadata on the rail pane.
- Refuse to close a pane whose ownership cannot be proved.
- Keep the rail optional; running `orc watch` normally remains supported.
- Do not modify the user's global status line or key bindings automatically.

Suggested optional bindings belong in documentation and `docs/tmux.conf`, not
in a mandatory installer.

### Collapsing the rail

Add a collapsed presentation so the rail can stay open without spending width:

```text
orc rail collapse
orc rail expand
orc rail toggle-collapsed
```

Collapse is a resize, not a teardown. The rail pane stays alive and keeps its
process; only its width changes.

```sh
pane=$(tmux list-panes -F '#{pane_id} #{@orc_rail}' | awk '$2=="1"{print $1}')
width=$(tmux display -p -t "$pane" '#{pane_width}')
if [ "$width" -gt 10 ]; then
    tmux resize-pane -t "$pane" -x 5
else
    tmux resize-pane -t "$pane" -x 64
fi
```

`resize-pane` delivers `SIGWINCH` to the rail process, which is the whole
mechanism. `orc watch` catches it, re-reads the terminal size, and redraws for
the width it now has:

- At full width, normal rows: state, ticket, stage, worker, title.
- At collapsed width, one state mark per row and nothing else.

Requirements:

- Never tear the pane down to collapse it. A restart loses scroll position,
  in-flight refreshes, and any selection state, and makes collapse feel
  destructive.
- Persist the collapsed flag on the window, for example `@orc_rail_collapsed`,
  so other scripts and a later reattach can read it.
- Do not force a minimum below what tmux will actually honor. Borders and
  layout constraints mean a two-column pane is not reliably reachable; five is
  a safe floor.
- Render the expand affordance inside the rail's own last row, or through
  `pane-border-format`, rather than requiring a keybinding to be discoverable.
- Redraw must be driven by `SIGWINCH`, not polling. A rail that repaints on a
  timer will flicker against agent output in neighbouring panes.

Mouse toggling is possible through `MouseDown1Pane` with `#{mouse_x}` and
`#{mouse_y}`, but hit-testing is manual arithmetic and should be treated as
optional polish rather than part of the first implementation.

## Seen state and multiple clients

The first version uses a conservative rule:

- an explicit Orc focus/attach action acknowledges completion;
- an explicit action in the Live rail acknowledges completion;
- passive tmux focus hooks may acknowledge only when the focused pane matches
  the exact agent instance;
- any attached client seeing the agent may mark it seen;
- CLI reads and terminal capture never mark it seen.

The rail must not guess which remote client should be switched. Standalone
focus continues to use the calling client unless an explicit client selector
is added later.

## Notifications

Authoritative transitions may route through Orc's existing notification layer:

- entering `blocked` requests human attention;
- entering unseen `done` announces completion;
- repeated identical events do not notify again;
- acknowledging an event does not emit another notification;
- screen-derived fallback may decorate the rail but does not send high-trust
  completion notifications by default.

tmux does not need a native notification surface. Orc may use its configured
OS, terminal, or command notification routes.

## Pane observation and title inference

Hooks report only for panes the user has installed them in. Agents started by
hand in an arbitrary pane report nothing, and those are exactly the ones a
rail is useful for. Orc should therefore discover agents by observation as
well as by registration.

tmux exposes everything required in one call:

```sh
tmux list-panes -a -F \
  '#{session_name} #{window_index} #{pane_id} #{pane_current_command} #{pane_title}'
```

That covers the whole server, not one session, and costs a single process per
poll rather than one per pane.

### Titles carry state, not just identity

Both target engines write their state into the OSC title, and it is a
stronger signal than screen content. Herdr's own detection ranks it that way:
in `src/detect/manifests/codex.toml`, the title rules sit at priority 1100 for
blocked and 1050 for working, while screen-content rules sit below them. The
title wins when both are available.

The observable vocabulary:

- **Working** — a braille spinner in the title, from the set
  `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`. Claude also cycles `· ✢ ✳ ✶ ✻ ✽`.
- **Blocked** — engine-specific wording; Codex writes `Action Required`.
- **Idle** — a non-empty title with no spinner.
- **Unknown** — no title, or a title that matches nothing.

Requirements:

- Poll `list-panes` on a bounded interval, not per pane, and not per frame.
- Treat title inference as `source=title`, ranked above `source=screen` and
  below any hook report.
- Never let inference overwrite newer hook metadata, and never let it satisfy
  a wait, complete work, or advance a stage.
- Strip the leading activity glyph before using a title as a display label,
  or the rail will flicker on every spinner frame. Herdr keeps both forms and
  lets the row choose.
- A pane whose title matches nothing publishes `unknown`, not a guess.

### Why this comes before screen scraping

Title inference needs no pane-content capture, no per-pane polling, and no
region model. It handles the common cases for both engines Orc targets. Screen
scraping should be treated as a third tier, reached only when hooks are absent
*and* the title is uninformative, rather than as the primary fallback.

## Conservative screen fallback

Screen fallback is implemented only after hook-backed lifecycle and title
inference both work, and only for cases neither covers.

- Rules live in versioned per-engine data files embedded in the binary.
- Rules inspect bounded bottom-of-screen regions with explicit priorities.
- Workspace overrides are optional and local; no detector catalog is fetched.
- Inferred metadata is labeled `source=screen`.
- Inference never overwrites newer hook metadata.
- Working-to-idle inference is debounced.
- Unclassifiable agents publish `unknown`, not a guessed settled state.
- Screen inference never satisfies `orc ctl agent wait`, completes work, moves a
  stage, or triggers automatic parking.

## Parking and restoration

Automatic reversible parking may consume only durable workflow state and
authoritative lifecycle. Before parking, Orc revalidates agent identity and
records enough information to resume or explain why resumption is unavailable.

Reconciliation classifies recorded agents as:

```text
live       exact target and instance match
resumable  provider session exists but no live instance matches
replaced   target exists with a different instance
orphaned   target and resumable provider session are unavailable
unknown    evidence is incomplete or contradictory
```

Restoration creates a new instance ID, stamps metadata before agent startup,
passes the provider resume identifier through the existing worker launch path,
and waits for the provider hook to register before declaring the agent live.

## Delivery plan

### Phase 1: identity and event contract

Status: implemented.

- Extend durable runtime state with agent and instance identity.
- Stamp and inventory identity metadata on exact tmux panes.
- Add event validation, sequencing, timestamps, and deduplication.
- Add stale, replaced, offline, and unknown classifications.
- Preserve compatibility with existing `STATE.yaml` files lacking agent IDs.

Acceptance: Orc can prove whether a recorded tmux pane still contains the
recorded agent instance and never targets a replacement.

### Phase 2: hook installer

Status: implemented.

- Add `internal/agenthooks` with Codex and Claude integrations.
- Add install and dry-run doctor flags.
- Publish lifecycle and attention through the event contract.
- Surface hook health and supported states in `orc doctor`.
- Add isolated configuration tests and `make verify-agents`.

Acceptance: supported agents emit observable, sequenced state transitions in
an isolated live tmux session.

### Phase 3: tmux lifecycle control

Status: implemented.

- Implement tmux `StateAgent`.
- Implement event-driven/polled `WaitAgent` with cancellation and timeouts.
- Include tmux agents in structured watch and aggregate status.
- Normalize backend error codes and JSON output.

Acceptance: state, wait, and watch work through the same `orc ctl` commands for
tmux and Herdr, with documented differences only where capabilities differ.

### Phase 4: prompt and stall detection

Status: implemented.

- Implement safe exact-pane prompt delivery.
- Require lifecycle movement and report stalled prompts.
- Detect replacement and exit during prompt/wait.
- Add explicitly confirmed prompting from `orc watch`.

Acceptance: Orc can submit a prompt containing spaces, quotes, newlines, and
shell metacharacters without interpolation or cross-pane delivery, then wait
for a recognized result.

### Phase 5: tmux rail and attention UX

Status: implemented.

- Add managed rail open/close/toggle behavior.
- Add age and stuck-state rendering.
- Add conservative seen-state handling.
- Route authoritative block and completion notifications.
- Document optional tmux bindings.

Acceptance: the rail behaves like a normal resizable tmux pane, survives
ordinary navigation, avoids duplicates, and never closes an unowned pane.

### Phase 6: fallback and reconciliation

- Add versioned screen rules and workspace overrides.
- Add debounce, precedence, and unknown-state behavior.
- Reconcile restored, replaced, resumable, and orphaned agents.
- Exercise restoration and automatic parking against authoritative state.

Acceptance: a missing hook degrades the display without corrupting lifecycle or
workflow state, and restored sessions cannot inherit stale agent identity.

## Verification strategy

### Unit tests

- state migration and round trips;
- ID generation and instance replacement;
- event validation, ordering, and deduplication;
- hook installer idempotence, malformed configuration, and dry-run behavior;
- attention precedence and age;
- lifecycle waits, cancellation, and stable error codes;
- prompt encoding and buffer cleanup;
- rail ownership and duplicate prevention;
- reconciliation classifications.

### Isolated tmux integration tests

- exact target validation across multiple panes and windows;
- pane replacement and stale target rejection;
- concurrent events and monotonic sequences;
- prompt delivery with hostile text;
- wait success, timeout, process exit, and server disappearance;
- rail creation, reuse, resizing tolerance, and safe close;
- multiple attached clients where practical.

### Agent-hook verification

`make verify-agents` installs both provider integrations into temporary config
roots, feeds representative Codex and Claude payloads through the installed
scripts, and verifies the resulting lifecycle options in an isolated tmux
server. It does not use personal configuration, authenticate an agent, or
submit a paid model request. A release smoke test may additionally exercise
installed real agents when authentication is already available.

The normal required gate remains:

```text
make check
```

Live-agent verification is a documented manual/release gate until it is stable
and credential-safe in CI.

## Compatibility and rollout

- Existing workspaces without agent identity continue to launch and attach.
- On the next controlled launch or verified resume, Orc backfills identity.
- Legacy attention hooks continue to decorate the UI during migration but do
  not provide structured lifecycle control.
- The tmux backend advertises `AgentControlBackend` only when the implementation
  is present; an individual agent with missing hooks returns `unknown` or a
  structured capability error.
- Herdr behavior and state files remain compatible.
- New commands are additive and do not rewrite user tmux configuration.

## Explicit non-goals

- Reimplementing tmux layout, rendering, copy mode, key tables, or plugins.
- Making Herdr mandatory or removing the Herdr backend.
- Treating terminal text as authoritative agent completion.
- Automatically approving provider hooks or permission prompts.
- Sending unconfirmed arbitrary prompts from the interactive Live rail.
- Building a hosted registry or required background service.
- Restoring arbitrary non-Orc tmux processes.
- Owning the user's global tmux theme or status line.

## Definition of done

Super Orc is complete when:

1. Codex and Claude can be configured through an idempotent Orc installer.
2. Orc assigns and validates durable agent and live instance identity.
3. tmux safely implements state, prompt, wait, watch, and capture through the
   backend-neutral control surface.
4. Stalled prompts, exited agents, replaced panes, missing servers, and
   timeouts produce distinct structured outcomes.
5. `orc watch` can run as a managed, mouse-resizable tmux rail without taking
   over the user's tmux configuration.
6. Completion and block notifications come from authoritative events.
7. Restoration cannot silently attach old Orc state to a different process.
8. Hookless agents degrade to clearly labeled presentation fallback.
9. `make check`, isolated tmux tests, and documented live-agent verification
   pass.
10. The same workspace remains usable with tmux alone, Herdr alone, or neither
    multiplexer available.
