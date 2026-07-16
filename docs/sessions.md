# Session inventory and recovery

`orc sessions` joins three deliberately separate sources:

1. `STATE.yaml` supplies durable ticket, workflow, stage, worker, and tmux identity.
2. Tmux supplies current pane liveness and Orc-owned metadata.
3. Claude and Codex local session files supply an optional live overlay: provider
   session ID, model, effort, state, context use, working directory, and last activity.

Provider telemetry never overrides durable Orc state. The readers extract only
metadata fields; prompt and response bodies are not retained or printed. Files
and JSONL line sizes are capped, and absence of a provider directory is normal.

Transcript parsing is bounded across each combined refresh:

- A process-local metadata cache keys each transcript by path, device/inode,
  size, and modification time.
- The first read scans a small head for identity plus a bounded recent tail for
  the latest model, lifecycle, and context metadata.
- A growing transcript resumes at its last complete-record byte offset.
- Truncation, replacement, or a malformed append invalidates the cached summary
  safely; incomplete records are retried after more bytes arrive.
- Claude and Codex share a 32 MiB and 250 ms parsing budget per refresh.

The cache never stores JSONL records or prompt/response bodies. It stores only
the extracted `Live` fields and cursor metadata. A new CLI process starts with
the bounded head/tail read; a long-running caller benefits from incremental
refreshes.

## Inventory

```sh
orc sessions
orc sessions --all
orc sessions --json
```

The default inventory includes:

- `managed`: a durable Orc feature with a configured tmux target. `running`
  distinguishes a current pane from a stopped target.
- `orphaned`: a pane marked with Orc metadata whose ticket no longer exists in
  the workspace.

`--all` also includes recent `unmanaged` Claude and Codex sessions. This makes
personal provider sessions visible without claiming Orc owns them.

For a single ticket, `orc status <ticket> --json` keeps the existing durable
state shape and adds a `live` field only when a running pane can be matched to
provider metadata.

## Provider identity correlation

Orc correlates a pane with provider telemetry in descending order of certainty:

1. An explicit `@orc_provider_session` marker together with the exact pane PID.
2. An exact provider session ID match.
3. An exact provider process PID match.
4. Engine plus CWD only when no explicit provider identity is present.

An ambiguous identity, or an explicit provider ID that cannot be found without
an exact PID, produces no live overlay. Orc does not silently attach telemetry
from another session in the same directory.

Tmux launches replace the pane shell with the provider process, so `pane_pid`
can be compared directly with providers that expose a PID. Resumed managed
sessions also carry:

- `@orc_provider_engine` and `@orc_provider_session` window/pane options.
- `ORC_RESUMED_FROM` in the tmux session and provider process environments.

Some providers use a new internal session ID while continuing to append to the
original resumable transcript. In that case, `provider_session_id` remains the
resumable identity and JSON output includes `observed_session_id` for the live
process identity. Orc merges process state with the original transcript's model
and context metadata. The `correlation` field reports `provider_id`, `pid`,
`provider_id+pid`, or `cwd`, so consumers can distinguish exact identity from
the compatibility fallback.

If a provider changes transcripts without exposing an exact new identity, as
some Claude `/clear` versions do, Orc keeps the previous exact association or
omits telemetry. It does not infer a rollover from CWD alone.

## Exact resume

```sh
orc sessions resume <provider-session-id> --dry
orc sessions resume <provider-session-id>
```

Resume accepts only a session discovered in recent local provider metadata. It
uses the recorded working directory, validates that directory and provider
binary, and passes argv directly without shell interpolation. Use `--engine`
when an ID is ambiguous and `--cwd` only when the recorded directory moved.

Sessions reported `active` or `working` are rejected by default. `--force`
exists for the case where provider metadata is stale; verify that the original
process is gone before using it.

## Park and unpark

Parking is intentionally two-step:

```sh
orc sessions park --dry
orc sessions park --yes

orc sessions unpark --dry
orc sessions unpark --yes
```

Only running, managed sessions with a matched Claude or Codex provider session
ID are parkable. Other panes are reported and left alone. Before stopping tmux,
Orc atomically writes a mode-`0600` snapshot under
`~/.local/state/orc/parked/`; the filename is scoped to the absolute workspace
path.

Unpark recreates the recorded session/window, resumes the provider with exact
argv, re-applies window and pane identity metadata, and updates
`runtime.tmux.pane`. It also passes `ORC_RESUMED_FROM` to the provider and records
the provider session ID on the tmux target. Successfully restored entries are
removed from the snapshot as they complete, so a partial failure can be retried
without losing the remaining recovery plan.
