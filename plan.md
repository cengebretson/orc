# orc plan

Single source of truth for unshipped work. Shipped work belongs in
`CHANGELOG.md`, not here.

## Release candidate

The core v1 feature scope is complete. The remaining release gates are
operational rather than feature work:

- Run the automated, fresh-workspace, and live-session checks in
  [`docs/release.md`](docs/release.md).
- Promote the intended entries from `[Unreleased]`, update `VERSION`, and verify
  the tag-driven release artifacts and checksums.

Direct prompt sending from `orc watch`, notifications, durable arbitrary labels,
remote pack lifecycle, and remote tmux control are explicitly post-v1.

## Later

Several entries below carry a **Prior art** line. Two projects solve the layer
directly beneath Orc and are worth borrowing from:

- **jmux** ([jarredkenny/jmux](https://github.com/jarredkenny/jmux)) — a fleet
  viewer that *wraps* tmux and composites its own chrome around it. Durable
  state lives in tmux user options rather than files. **AGPL-3.0: borrow the
  design, never the code.**
- **herdr** ([herdrdev/herdr](https://github.com/herdrdev/herdr)) — an agent-aware
  multiplexer that *replaces* tmux, with a client/server model, a JSON socket
  API, an event bus, and per-agent detection manifests. **Apache-2.0**, so code
  reuse is possible where it makes sense.

Neither is a competitor. Both own the live layer — what an agent is doing right
now — and neither has any notion of a ticket, a stage, a required artifact, or
history. Orc's durable state and workflow policy remain its own. What transfers
is everything below the workflow: how live agent state is observed, how one
agent dispatches another, and how a session surface is driven.

### Agent completion notification

Status: Implemented. `settings.notify` supports filtered `blocked`, `complete`,
`error`, and `all` events; current transitions emit blocked and complete after
their state writes, with templates, `ORC_*` environment, timeout, and
best-effort warning behavior.

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

### Record which multiplexer produced a session

Status: Implemented. New launches write `runtime.mux` with backend and exact
workspace/tab/pane IDs; legacy `runtime.tmux` remains a read-only compatibility
fallback.

Leftover from the `mux.Backend` seam, which shipped without it.
`runtime.tmux.session` in `STATE.yaml` still names its backend in the field
itself, so a workspace cannot say a session came from anything but tmux.

- Keep `runtime.tmux.session` readable forever; old state must not need
  migrating.
- Introduce a backend-neutral target only as the first slice of the native
  herdr backend below, when its actual identifiers make the required shape
  concrete. One likely shape is `runtime.mux: {backend, workspace, tab, pane}`.
- New writes record `mux.Backend.Name()` alongside the backend's opaque target
  identifiers. Labels are presentation; they are not durable identity.
- Keep reading `runtime.tmux` as a compatibility fallback and do not rewrite old
  state merely because it was loaded.

Effort: Small.

### Add a native herdr multiplexer backend

Status: First vertical slice, native worktree create/open, task-cell layouts,
and native transition notifications implemented. Launch, exact identity, agent
lifecycle inventory, attach/focus, dashboard/watch selection, archive cleanup,
sidebar metadata, worktree-backed workspace creation, optional test/watch
panes, and blocked/completed alerts are wired through `--mux herdr`. The
remaining extension ideas stay future work. Live
disposable-workspace smoke tests verified Codex launch, exact target
persistence, lifecycle inventory, sidebar tokens, attach targeting, owned
archive cleanup, Herdr's native worktree create/open response shapes, and the
split/rename/run topology used by task cells.

Herdr is a strong second implementation of `mux.Backend`: it owns the live
terminal layer Orc deliberately does not, recognizes agent lifecycle directly,
and exposes structured workspace, tab, pane, agent, worktree, notification, and
terminal operations through its CLI and socket API. The conceptual mapping is
clean:

| Orc | herdr |
|-----|-------|
| ticket session | workspace |
| workflow stage/window | tab |
| worker process | recognized agent pane |
| watch/tests | sibling panes |
| live attention | agent lifecycle state |
| ticket worktree | worktree-backed workspace |

Do not implement this as string substitution around the tmux backend. The
current interface still encodes tmux assumptions that herdr does not share:
named sessions, windows addressable by name, metadata on a window, and an
environment that can be mutated after session creation. Herdr returns opaque
workspace/tab/pane IDs, reports metadata on workspaces and panes, and supplies
environment during workspace/tab/pane creation.

Prerequisite interface work:

- Replace `CreateSession(... ) error` with a creation result that returns the
  exact workspace, tab, and root-pane identifiers Orc must persist.
- Pass creation environment as options instead of relying on
  `SetSessionEnvironment` after creation. This matters for `ORC=1`, ticket
  identity, and `ORC_RESUMED_FROM`.
- Introduce a backend-neutral target value rather than passing loose
  `session`, `window`, and `pane` strings through every call.
- Keep tmux as the default backend and preserve foreground fallback. Selecting
  herdr must be additive; an Orc workspace remains usable when herdr is absent.
- Remove remaining hidden tmux defaults after backend selection is centralized;
  `sessionlist`, watch, dashboard, attach/focus, parking, archive, and resume
  must all receive the selected backend.

First vertical slice:

1. Add `internal/herdr`, executing the installed CLI and decoding its JSON
   responses. The installed binary is the syntax authority; never derive an ID
   from a label or sidebar position.
2. `orc next --mux herdr` creates or reopens one workspace per ticket and one
   tab per stage, then records the returned workspace/tab/pane IDs.
3. Start the configured engine with `herdr agent start` and submit the stage
   prompt with `herdr agent prompt`. Use the agent surface, not raw pane input,
   whenever herdr recognizes the worker.
4. Populate `orc sessions`, status, watch, and dashboard from herdr workspace,
   pane, and agent inventory. Keep transcript telemetry only for fields herdr
   does not supply, such as model detail or context consumption.
5. Attach/focus the exact stored pane. From inside herdr, focus the target;
   from outside, resolve its terminal and attach explicitly rather than acting
   on whichever workspace another client has focused.
6. Park/archive may close only a workspace stamped as Orc-owned. A matching
   label is never proof of ownership.

Lifecycle mapping:

| herdr state | Orc live meaning |
|-------------|------------------|
| `working` | active |
| `blocked` | blocked / input required |
| `done` | done or ready for review |
| `idle` | ready for another prompt |
| `unknown` | present but unclassified; never completion |

Herdr distinguishes `idle` from `done` using whether completed background work
has been seen. Preserve that distinction in structured output even if a compact
rail rolls both into a lower-urgency presentation. Durable workflow state in
`STATE.yaml` still wins; live lifecycle never advances a stage by itself.

#### Enrich herdr's sidebar instead of replacing it

Herdr owns sidebar rendering, navigation, selection, built-in lifecycle icons,
and the agent/space sections. Orc should not ship a competing sidebar renderer.
Herdr's configurable sidebar rows and custom metadata tokens are the native
extension point:

- Workspace tokens: `ticket`, `workflow`, `repository`, `branch`, `stage`, and
  `next_action`.
- Pane tokens: `ticket`, `stage`, `worker`, `model`, `context_pressure`, and
  `next_action`.
- State labels: Orc-specific text such as `awaiting review`, `QA blocked`, or
  `ready to merge`, while herdr retains the underlying lifecycle state.
- Report every value with `source=orc`. Use monotonic sequence numbers and TTLs
  for ephemeral values so stale live metadata cannot outlive its producer;
  durable identity continues to come from `STATE.yaml`.

An optional user-owned config snippet can render the values:

```toml
[ui.sidebar.agents]
rows = [
  ["state_icon", "$ticket", "$stage"],
  ["agent", "$worker"],
]

[ui.sidebar.spaces]
rows = [
  ["state_icon", "$ticket", "$workflow"],
  ["branch", "$next_action"],
]
```

Document the snippet first. If Orc later offers `orc herdr configure`, make it
explicit, previewable with `--dry-run`, and conservative with the user's
`~/.config/herdr/config.toml`; never silently replace their sidebar rows. For a
fully custom ticket dashboard, keep `orc watch` as a herdr pane or popup. That
is the right surface for arbitrary Bubble Tea UI.

After the vertical slice is stable:

- Implemented: use `herdr worktree create/open` to create or reopen
  unambiguous ticket workspaces, persist the checkout before agent launch, and
  preserve repository-owned `worktree_setup` commands. Removal continues
  through Orc's existing ownership and archive safeguards.
- Implemented: offer a metadata-owned task-cell layout with the agent pane plus
  optional test and watch panes. Repeated launches reuse exact Orc-owned panes
  and leave user-created panes alone.
- Implemented: send Herdr-native notifications when work blocks or completes,
  with request/done sounds and best-effort failure behavior.
- Reuse herdr's blocking prompt/wait and stall detection underneath `orc ctl`
  rather than rebuilding those semantics for this backend.
- Defer named-session, remote attach, and event-stream optimization until the
  local lifecycle is correct.

Acceptance criteria for the first slice:

- `orc next --mux herdr` starts one configured Claude or Codex worker and
  records exact opaque target IDs.
- `orc sessions --json` distinguishes working, blocked, idle, done, unknown,
  stopped, orphaned, and unmanaged work without inferring completion from
  screen text.
- `orc attach` targets the recorded pane even when user focus has moved.
- Sidebar tokens update from Orc metadata without moving workflow authority out
  of `STATE.yaml`.
- No herdr server is an empty live inventory, not a dashboard failure; the
  normal foreground fallback remains available.
- Orc never closes, parks, or adopts a herdr workspace it cannot prove it owns.
- Existing tmux workspaces and old `runtime.tmux` state continue to work.

Effort: Large.

### Ship the agent hook installer

Today the live attention layer depends on an external `tmux-attention` tool the
user has to discover and install themselves. `docs/watch.md` is explicit that the
integration "is optional and is a no-op when `tmux-attention` is not installed" —
which means the attention column in `orc watch` and the dashboard Live tab is
dark for anyone who has not gone looking. Orc should install the emitter itself.

```
orc doctor --install-agent-hooks     # install into every agent CLI present
orc doctor --install-agent-hooks --dry-run
```

Behavior:

| Outcome | When |
|---------|------|
| `installed` | hooks written into an agent that had none |
| `already up to date` | hooks present and current; nothing written |
| `skipped` | agent not installed on this machine |
| `failed` | config unreadable or unparseable; nothing written |

Implementation notes:

- Add `internal/agenthooks` with one integration per engine, mirroring the
  `engine` values already parsed in `internal/workers` (`claude`, `codex`,
  `cursor`). Each integration reports `isPresent`, `detect`, `install`,
  `uninstall`, and the set of states it can actually observe.
- Emit the same `@agent_attention` values Orc already defines in
  `internal/tmux/tmux.go` (`input`, `blocked`, `review`, `done`) so the existing
  watch and dashboard readers light up with no consumer changes.
- Resolve the Claude config directory from `$CLAUDE_CONFIG_DIR` **per call**,
  falling back to `~/.claude`. Resolving once at startup, or assuming `~/.claude`
  outright, installs hooks that never fire for anyone who has relocated their
  config — and reports success while doing it.
- Never overwrite an unparseable config. It is the user's file; surface the error
  and write nothing.
- Codex gates hooks behind `[features] hooks = true` and requires the user to
  approve each hook on first launch. Set the feature flag, then say plainly that
  Codex will ask and that state stays blank until it is approved. Do not forge
  the approval hashes — that is a security decision, not a convenience.
- Idempotent: re-running writes nothing when hooks are already current.
- Surface installed/absent hook state as a `doctor` check so `orc doctor` reports
  the gap even when the user never runs the installer.

Verification (`make verify-agents`, manual gate, not part of `make test`):

- Unit tests cannot cover the link that matters — whether a real agent actually
  invokes the emitter. That link spans two projects and breaks silently: an
  upstream rename of a hook event leaves every test green and the rail blank.
- Drive the real agent binaries in an isolated tmux server against a sandboxed
  config home and assert the states actually observed.
- Run each agent in two phases. A one-shot run ends with the completion and
  session-end hooks firing back to back, so the terminal state exists for
  milliseconds and no sampler catches it — indistinguishable from a broken hook.
  Phase one suppresses session-end to prove the transition; phase two installs
  the full set to prove teardown clears state.
- An agent that cannot authenticate in the sandbox is `SKIP`, not `FAIL` — it
  never takes a turn, so the completion hook legitimately never fires. Claude
  Code hits this whenever real credentials live in the macOS Keychain.

Prior art: jmux (`jmux --install-agent-hooks`, `bun run verify:agents`).

Effort: Medium.

### Derive attention for agents that do not report it

Pane-scoped reading and the window rollup have shipped. What remains is the
tier that *derives* a state by reading the screen, for agents whose hooks report
nothing, plus the two display uses of the timestamp the rollup now returns.

- Surface `@agent_attention_since` in the rail: age a marker, and flag an
  implausibly long `blocked` as likely stuck rather than merely waiting.
  `mux.RollUpAttention` already returns the time; nothing displays it yet, and
  `mux.Backend.Attention` does not expose it — grow the interface when the rail
  actually needs it, not before.
- Mark derived state as derived. Anything inferred from reading a pane rather
  than reported by a hook gets `@agent_attention_source screen`, and the derived
  tier may never overwrite a state the agent reports for itself. A guess must
  stay distinguishable from a fact.
- Consider whether `internal/sessionlist` should roll up too. It reports the
  matched pane's own state, which is right for a pane-level listing, but it can
  disagree with the window rollup the rail shows for the same ticket. Decide
  deliberately rather than letting the two drift.

If a derived tier is built at all, it belongs in a **data file, not Go code**.
Screen signatures rot — an agent ships a UI change and the detector silently
starts lying — and a rotted signature that lives in code can only be fixed by
cutting a release.

- Rules in versioned per-engine files, not `switch` statements: an id, a target
  state, a priority, and a **region** to match within (terminal title, the last
  N non-empty lines, everything after the last horizontal rule) rather than the
  whole screen. Region scoping is what stops a rule matching an agent's own
  transcript of a previous prompt.
- Carry a schema version on each file and refuse to load one written against a
  newer engine than this binary understands.
- Debounce before declaring an agent finished. A working→idle transition should
  require several confirmations across a few hundred milliseconds; a single
  quiet sample is a pane between tool calls, not a completed turn.
- Add an explicit `unknown`: an agent is present but unclassifiable. It must
  never be treated as completion, and it must be visibly different in the rail
  from "no agent here."
- Orc needs two engines, not twenty. Keep the scope to the `engine` values
  `internal/workers` already parses.

Prior art: jmux (pane scope, documented rollup, `@jmux-agent-source screen`);
herdr (`src/detect/manifests/*.toml` — versioned, priority-ordered,
region-scoped rules with `min_engine_version`, and a pending-idle confirmation
counter before publishing `idle`).

Note what Orc should *not* copy: herdr fetches its manifest catalog over the
network from a hosted URL, which is what lets it track UI changes across
nineteen agents without shipping a binary. That is a reasonable trade for a
project whose whole job is agent detection, and a bad one for Orc — it puts a
network service in the middle of local state reads. Ship the files in the
binary and let a workspace override them on disk.

Effort: Small (timestamp display) / Medium (the derived tier).

### `orc ctl` — agent-facing control surface

Orc has `orc mark` (an agent writes its own durable state) and `orc status
--json` (a human or script reads it). It has nothing that lets an agent
**dispatch and supervise a sibling agent** — no way to fan out, chain, or block
until another agent finishes. The parts exist (`orc jit --tmux`, tmux metadata
stamping, `internal/sessionlist`, `internal/telemetry`); the surface does not.

```bash
orc ctl status                              # one snapshot: what exists, what needs me
orc ctl agent state [--ticket T]            # structured live state
orc ctl agent watch [--ticket T]            # JSONL stream of transitions
orc ctl agent prompt --ticket T "<text>" --wait --timeout 120s
orc ctl agent wait --ticket T --until blocked --timeout 120s
orc ctl session capture --ticket T          # screen text, when text is what you need
```

Implementation notes:

- **`prompt --wait` is the primitive to build first**, ahead of the stream. Orc's
  launches are request/response shaped — send a prompt, block until the agent
  settles, return — so a blocking call fits the actual usage better than asking
  every caller to consume a stream and implement its own state machine. Without
  `--until` it waits for the first settled `input`/`blocked`/`review`/`done`.
- **Ship a stall detector with it.** A prompt sent to a non-working agent that
  produces no observed state change within a few seconds must return a distinct
  `stalled` error rather than blocking until timeout. An unattended orchestrator
  that cannot tell "still thinking" from "the keystrokes went nowhere" will hang
  for the full timeout on every dropped prompt, and the failure looks identical
  to slow work.
- Send text and the Enter keystroke **atomically**, honoring the pane's live
  bracketed-paste mode. Two separate sends race with the agent's own redraw.
- `agent watch` stays as the second piece: one JSON object per line, per
  transition, until interrupted. It covers the fan-out case where a supervisor
  is tracking several tickets and cannot block on any one of them.
- Waits track **lifecycle state, not an individual turn**. If the agent is
  already working when the prompt lands, completion of that active turn may
  satisfy the wait. Document this rather than pretending the wait is turn-scoped.
- All output structured JSON on stdout, errors as JSON on stderr. No human
  formatting in `ctl` — `orc status` and the dashboard already own that.
- Target the exact agent pane recorded by `SetPaneMetadata` (`@orc_agent`), never
  the session's active pane, which drifts after splits.
- Export `ORC=1` plus the ticket into launched sessions so an agent can detect it
  is under Orc without being told.
- Ship a skill file with the binary that documents the surface. Workspace policy
  belongs in `AGENTS.md`/`RULES.md`, but a skill travels with the binary and
  works in a session that is not in a workspace yet.
- Write the skill as **rules about not doing the wrong thing**, not as a command
  list. The command list is `--help`; the skill's job is the judgment around it:
  prefer structured state over screen scraping (reach for a capture only when the
  screen *text* is the thing you need, never to infer lifecycle); target an
  explicit ticket or pane rather than whatever is focused, because focus may
  belong to the human; parse identifiers out of JSON responses instead of
  deriving them from examples; never close or kill work you did not create;
  never stop the session you are running inside. Point at `--help` as the
  authority for syntax so the skill cannot drift out of date with the binary.
- Keep the boundary honest: `ctl` lets an agent control *work*, never the
  human's *view*. No command may retarget what the user is looking at.
- `STATE.yaml` stays authoritative. `ctl` reads live state and sends input; it
  does not become a second way to mutate durable state — that is still
  `orc mark`.

Prior art: jmux (`jmux ctl`, `skills/jmux-control.md`); herdr (`herdr agent
prompt --wait`, `agent wait --until`, the `agent_prompt_stalled` error, and
`skills/herdr/SKILL.md`, which is the better of the two skill files by some
margin).

Effort: Large.

### Parking that reverses itself

`orc sessions park/unpark` is a manual snapshot-and-stop: useful, but it is a
stash. The work still lives in the operator's head, because nothing brings it
back. Parking is only trustworthy when it undoes itself.

Config shape:

```yaml
settings:
  parking:
    auto_park: [paused]
    wake_on: [status_change, attention, stage_change]
```

Wake conditions, all from inputs Orc already has:

| Condition | Source |
|-----------|--------|
| `status_change` | `STATE.yaml` status leaves the parked value |
| `attention` | `@agent_attention` becomes `input`, `blocked`, or `review` |
| `stage_change` | `STATE.yaml` stage advances |

Implementation notes:

- Extend `internal/parking` from snapshot storage to policy: which tickets are
  parked, why, and what would wake them.
- Collapse parked work into a single `Parked (n)` row at the bottom of the
  `orc watch` rail and the dashboard Live tab, expandable — never hidden
  outright.
- Nothing is destroyed. The tmux session, worktree, and scrollback stay exactly
  as they were; parking is a display and attention decision only.
- A woken ticket comes back **flagged**, so it is visibly different from work
  that never left.
- Auto-park stays opt-in. A ticket that parks itself and never returns is worse
  than no parking at all, so ship the wake path before the park path.

Prior art: jmux (status-driven parking with automatic un-parking).

Effort: Medium.

### Ship Orc as a herdr plugin

Cheap, additive, and does not require the multiplexer abstraction above — a
herdr plugin is a directory with a `herdr-plugin.toml` manifest and argv
commands herdr can launch. Any language, no SDK: "the entire Herdr CLI is the
plugin API." An existing Go binary qualifies as-is.

The plugin surfaces Orc inside herdr's UI without Orc taking a dependency on it.

```toml
id = "orc.workspace"
name = "Orc"
version = "0.1.0"
min_herdr_version = "0.7.0"
platforms = ["linux", "macos"]

[[panes]]
id = "rail"
title = "Orc"
placement = "split"
command = ["orc", "watch"]

[[actions]]
id = "next"
title = "Launch next stage"
contexts = ["workspace"]
command = ["orc-herdr-action", "next"]

[[link_handlers]]
id = "ticket"
title = "Start work on this ticket"
pattern = "<tracker issue URL pattern>"
action = "work"

[[events]]
on = "worktree.created"
command = ["orc-herdr-action", "on-worktree-created"]
```

What each surface buys:

| Surface | Value |
|---------|-------|
| `[[panes]]` | `orc watch` becomes a native herdr pane — the rail without a dedicated terminal |
| `[[actions]]` | `orc next` / `mark next` / `archive` from herdr's own command surface |
| `[[link_handlers]]` | click a tracker URL in any pane → `orc work <ticket>` scaffolds the feature |
| `[[events]]` | react to real events: `worktree.created`, `pane.agent_status_changed`, `pane.exited` |

Implementation notes:

- The heavy lift is a small `orc-herdr-action` shim, not changes to Orc. It
  reads `HERDR_PLUGIN_CONTEXT_JSON` (workspace, tab, focused pane, worktree,
  agent, selected text, clicked URL), maps it onto a ticket, and shells out to
  the existing Orc commands. Keep it in `packaging/` or its own small directory —
  it is glue, not core.
- Call herdr back through `HERDR_BIN_PATH`, never a bare `herdr`, so the plugin
  works across Unix sockets and Windows named pipes.
- **`STATE.yaml` does not move.** herdr offers `HERDR_PLUGIN_STATE_DIR` for
  plugin-owned durable state; Orc must not use it for workflow state. The
  workspace stays the source of truth, the plugin stays a surface over it, and
  an Orc workspace must remain fully usable with herdr uninstalled. At most the
  state dir holds herdr-local view preferences.
- `min_herdr_version` pins the oldest herdr exposing the manifest fields used;
  herdr refuses to link a plugin whose minimum is newer than the binary.
- Ship it as its own repo or a subdirectory, not as a hard dependency. Nothing
  in Orc proper should import or assume herdr.
- Plugin commands run as the user with no sandbox, which is the normal bargain
  for editor and shell extensions, but means the shim should stay small enough
  to read in one sitting.

This is the honest answer to "should Orc integrate with a fleet viewer": not by
becoming one, and not by coupling to one — by publishing a thin adapter and
letting the workspace stay portable.

Effort: Small.

### Emit jmux-compatible pane state

Small interop win, near-zero cost. jmux's state protocol is deliberately open —
any hook, script, or CI callback can report into it. If Orc stamps
`@jmux-agent-state` and `@jmux-agent-state-since` alongside its own
`@agent_attention`, jmux's sidebar and Command Center light up for Orc-managed
sessions with no work on either side.

- Map `input`/`blocked` → `waiting`, active → `running`, `done`/`review` →
  `complete`, and set `@jmux-agent-kind` from the worker's `engine`.
- Write it in the same place `SetPaneMetadata` already writes, behind
  `settings.emit_jmux_state` (default off).
- Costs nothing when jmux is not installed — they are inert tmux user options.

Scope limit: this helps jmux and anything else reading tmux user options. It
does **not** reach herdr, which is not tmux and has no user-option store — that
path is the plugin entry above.

Effort: Small.

### CONTEXT.md — a domain glossary

`CLAUDE.md`'s "Deliberate Divergences" table records what changed from the
original design. That is history, not vocabulary. Orc has no file that says what
its words currently *mean* — stage, loop stage, worker, pack, feature folder,
runtime, jit, park, attention — and it is a project whose primary readers are
agents deciding what to do next. Two sessions using "stage" and "loop stage"
differently is a real failure mode.

- One file, `CONTEXT.md`: terms and their meaning, independent of implementation.
- Rule at the top: when a term's meaning sharpens during design, update it here.
- Add short ADRs under `docs/adr/` only for decisions that keep getting
  re-litigated — durable state in files, policy in files not code, stage-assigned
  workers by default.
- Keep it lean. jmux runs roughly 1:1 markdown to code; Orc is nearer 1:6 and
  that is the healthier ratio. Copy the glossary, not the volume.

Effort: Small.

## Future ideas

- Explicit, confirmed prompt sending from `orc watch` to an exact agent pane.
- Structured confirm/choice/text human responses.
- Durable arbitrary labels and `--label key=value` filters.
- Remote tmux client selection for standalone watch/focus processes.
- User profiles for personal defaults across workspaces.
- Remote pack installs.
- Pack update and uninstall.
- Pack registries.
- Trust, signing, or provenance beyond local path and digest metadata.
- Per-run log capture for post-mortem debugging of unattended runs.
- Per-worker cost attribution built on `orc report`.
- Homebrew tap, once binary releases have settled.
- Workflow stages mapped onto an external tracker's statuses. Orc's stages
  already *are* the user's model — the missing half is the vocabulary bridge to a
  tracker with twenty-five statuses named for someone else's process. If a
  tracker adapter ever lands, the shape is: each status gets exactly two
  independent settings — which stage it belongs to, and whether it parks — and
  every write-back to the tracker defaults to off. (Prior art: jmux.)
- Derive dashboard and watch colors from the terminal's own palette instead of
  hardcoding them in `internal/workspaceui/styles.go`, so Orc looks native in
  light and dark terminals with no configuration. Removes a config axis.
  (Prior art: jmux.)

## Not now

- No guided `orc setup` command; setup remains agent-driven through `SETUP.md`,
  consistent with Orc's agent-first operating model.
- No hosted Orc service.
- No required public pack publishing.
- No generated workspace dependency on a central registry.
- No broad project-management features until the agent workflow loop is tighter.
- No terminal compositing, and no grid of live drivable agent panes. That
  capability follows from owning the terminal surface and running tmux inside a
  pty you own; Orc is a Bubble Tea application running *inside* tmux and can
  never composite live panes from that position. The same layer boundary is why
  dashboard portraits need `allow-passthrough on`. This is an architectural fork,
  not a feature gap — chasing it would mean rewriting Orc as a multiplexer.
  Compose with a fleet viewer instead; see "Ship Orc as a herdr plugin" and
  "Emit jmux-compatible pane state".
- No replacing tmux as the default substrate. herdr is a better substrate on the
  merits — agent lifecycle as a primitive, a blocking wait, an event bus, native
  resume across a dozen agents — but tmux is on every machine and every server,
  and Orc's hard requirement is portability. Build the seam (see "Abstract the
  multiplexer behind an interface") so a herdr backend stays cheap to add later;
  do not make one the default, and do not make either one required.
