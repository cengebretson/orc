# TOOLS.md

This file records verified ways to access external systems and preferred local
tools. It does not grant permission; read `RULES.md` before mutating anything.
Repository-specific build and test commands belong in that repository's own
instructions or in `orc.yaml` agent hints.

## External Systems

Record exact, working access methods. Ticket retrieval is configured here;
repository routing is configured in `orc.yaml`.

| Capability | System | Preferred access | Fallback | Scope or notes |
|---|---|---|---|---|
| Ticket read/write | <!-- Jira, Linear, GitHub Issues, none --> | <!-- verified MCP/app/CLI --> | <!-- verified fallback or none --> | <!-- projects/keys --> |
| Source control and PRs | <!-- GitHub, GitLab, Bitbucket --> | <!-- verified MCP/app/CLI --> | <!-- verified fallback or none --> | <!-- orgs/default target --> |
| CI/CD | <!-- system or none --> | <!-- verified access --> | <!-- fallback --> | <!-- read/trigger scope --> |

Never record tokens or secret values here. Name the authentication mechanism
only when agents need it, such as “GitHub app” or “CLI OAuth profile.”

## Engine Integrations

List integrations that setup verified as available to each engine. Do not list
aspirational or merely installed integrations as usable.

| Engine | Available MCP servers, apps, plugins, or skills |
|---|---|
| Claude | <!-- verified names or none --> |
| Codex | <!-- verified names or none --> |

Prefer an available structured integration over scraping text or constructing
raw API calls. If the preferred access fails, report the fallback before using
it.

## Local CLI Tools

Setup should keep only tools verified on `PATH` and add workspace-specific
wrappers when relevant.

| Tool | Use |
|---|---|
| `rg` | Recursive text search |
| `fd` | File discovery |
| `jq` | JSON parsing and transformation |
| `yq` | YAML, TOML, and JSON queries |
| `ast-grep` | Syntax-aware code search and rewrites |
| `shellcheck` | Shell script validation |

Use repository-defined package managers, task runners, and validation commands
instead of inventing replacements. If a listed tool is unavailable, use a safe
installed fallback and note the substitution.

## Local Git

Local Git is appropriate for inspection, ticket worktrees, diffs, and other
actions allowed by `RULES.md`. If `orc.yaml` defines `repos[].worktree_setup`,
use the command Orc supplies rather than raw `git worktree add`.

Remote Git and pull-request actions use the verified source-control access above
and remain subject to `RULES.md`.
