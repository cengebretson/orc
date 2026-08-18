# Events

`orc events` exposes changes already visible in Orc's immutable workspace
snapshot. It is a read-only integration surface: `STATE.yaml` and the rest of
the feature folder remain the source of truth.

```bash
orc events                    # print a human-readable baseline, then exit
orc events --json             # print the baseline as JSONL, then exit
orc events --follow --json    # print the baseline, then follow changes
orc events --follow --interval 2s
```

JSON output is newline-delimited JSON. Each line is a complete event, so a
consumer can process the stream without buffering an array.

## Event types

| Type | Emitted when |
| --- | --- |
| `feature.changed` | A managed feature is added or removed, or its durable feature metadata changes. |
| `attention.changed` | The feature's resolved attention state changes. |
| `session.changed` | Its matched managed session or live provider telemetry changes. |
| `stage.changed` | Its resolved stage or assigned worker changes. |

On startup, Orc emits one `feature.changed` event for every current feature,
with `before` set to `null`. This baseline lets a follower build current state
before applying later changes. Removing a feature emits `feature.changed` with
`after` set to `null`.

A single snapshot refresh can emit more than one event for a feature. Events
are ordered by feature key, then by type in this order:
`feature.changed`, `attention.changed`, `session.changed`, `stage.changed`.

## JSON schema

Every event contains the complete normalized item before and after the change,
not just the changed field:

```json
{
  "type": "stage.changed",
  "at": "2026-08-18T17:01:00Z",
  "ticket": "ORC-123",
  "feature_dir": "/workspace/features/ORC-123-events",
  "before": {
    "feature": {
      "ticket": "ORC-123",
      "slug": "ORC-123-events",
      "feature_dir": "/workspace/features/ORC-123-events",
      "status": "active",
      "workflow": "default:standard",
      "archived": false,
      "has_issues": false
    },
    "stage": {
      "name": "develop",
      "label": "Develop",
      "worker_id": "default:bob",
      "worker_name": "Bob"
    },
    "attention": {"state": "working", "source": "hook"},
    "session": {
      "running": true,
      "backend": "tmux",
      "workspace": "ORC-123-events",
      "tab": "develop",
      "engine": "codex",
      "lifecycle": "working",
      "lifecycle_source": "hook"
    }
  },
  "after": {
    "feature": {
      "ticket": "ORC-123",
      "slug": "ORC-123-events",
      "feature_dir": "/workspace/features/ORC-123-events",
      "status": "active",
      "workflow": "default:standard",
      "archived": false,
      "has_issues": false
    },
    "stage": {
      "name": "review",
      "label": "Review",
      "worker_id": "default:zach",
      "worker_name": "Zach"
    },
    "attention": {"state": "working", "source": "hook"},
    "session": {
      "running": true,
      "backend": "tmux",
      "workspace": "ORC-123-events",
      "tab": "review",
      "engine": "codex",
      "lifecycle": "working",
      "lifecycle_source": "hook"
    }
  }
}
```

The four nested projections are stable domains:

- `feature`: ticket, slug, feature directory, status, workflow, archive and
  validation state, plus required artifacts
- `stage`: canonical name, display and loop labels, and assigned worker
- `attention`: resolved attention state, source, and observation time
- `session`: backend-neutral target and running state plus Orc agent identity,
  lifecycle evidence, matched provider session, model, activity, and context
  usage when available

Optional fields are omitted when unknown. `before` or `after` is `null` only
for feature creation or removal.

## Scope and failure behavior

The stream covers managed feature items in the workspace snapshot. It does not
emit standalone events for unmanaged or orphaned sessions, and it does not
include prompts, transcripts, or message bodies. Live session information is
an observation layered onto durable feature state.

`--follow` reloads the snapshot every five seconds by default. If a refresh
cannot load the workspace, the command exits non-zero instead of silently
continuing with stale state. Interrupting the command exits cleanly.
