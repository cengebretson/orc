---
id: worker-id
name: Worker Display Name
engine: claude
model: claude-sonnet-4-6
args:
  effort: medium
---

# Worker Display Name

## Role

<!-- One sentence describing this worker's primary responsibility -->

## Best For

- 
- 

## Avoid

- 
- 

## Permission Boundaries

Ask before:

- Installing dependencies
- Rewriting Git history
- Writing to external systems (Jira, GitHub, CI)
- Starting background agents

## Launch and Prompt

`orc` builds the launch command and the prompt automatically — you do not specify
them here. The launch command is derived from the `engine` field above (plus
`model` and `args`), and the prompt is assembled from the ticket's `STATE.yaml`
and the current stage file. Namespaced stage IDs map to nested paths, so
`default:develop` is `stages/default/develop.md`. Set `engine` correctly and the rest follows;
run `orc next <ticket> --dry` to see exactly what will be launched.
