# TOOLS.md

## Tool Policy

Read this file before choosing commands, MCP servers, scripts, or external apps.
For repo-specific tools (package manager, test runner, docker), check the repo's
own instruction files.

---

## Ticket System

<!-- Configure your ticket system here. Agents will use this to fetch and update tickets. -->

- **System:** <!-- jira | linear | github-issues | shortcut | none -->
- **Access:** <!-- e.g. use the `jira-mcp` MCP server -->
- **Project keys:** <!-- e.g. FLYWL, DLOS -->
- **Ticket URL format:** <!-- e.g. https://yourcompany.atlassian.net/browse/{ticket} -->

---

## Source Control

<!-- Configure your source control here. Agents will use this to open PRs and check CI. -->

- **System:** <!-- github | gitlab | bitbucket -->
- **Access:** <!-- e.g. use the `github-mcp` MCP server -->
- **Org / repo:** <!-- e.g. myorg/myrepo -->
- **Default branch:** <!-- e.g. main -->
- **PR target:** <!-- e.g. main or release branch -->

Agents may use local `git` commands for read-only inspection and ticket worktree
creation. Use the configured source-control access above for remote actions such
as opening PRs, reading CI status, or posting comments.

---

## CLI Tools

Prefer these tools over naive alternatives. They are faster, safer, and produce
output that is easier to parse and reason about.

| Use this   | Instead of      | For                                                  |
|------------|-----------------|------------------------------------------------------|
| `rg`       | `grep`          | Fast recursive text search                           |
| `fd`       | `find`          | File finding with simpler syntax                     |
| `jq`       | manual parsing  | Parsing and transforming JSON                        |
| `yq`       | manual editing  | Reading and editing YAML, TOML, JSON in pipelines    |
| `ast-grep` | `grep` / regex  | Searching or rewriting code by structure             |
| `sd`       | `sed`           | Find-and-replace with clean syntax                   |
| `bat`      | `cat`           | Viewing files with syntax highlighting               |
| `delta`    | `diff`          | Reviewing diffs with syntax highlighting             |

---

## Git and Worktrees

Use worktrees for ticket implementation so the main repo checkout stays clean.
Unless a repo-specific instruction says otherwise, create worktrees under:

```
worktrees/<repo-name>/<ticket-slug>/
```

Recommended local commands:

| Command | Purpose |
|---------|---------|
| `git status --short` | Check for local changes |
| `git branch --show-current` | Confirm current branch |
| `git worktree list` | See existing worktrees |
| `git worktree add <path> -b <branch>` | Create a ticket worktree and branch when no repo-specific setup command is configured |
| `git diff --stat` | Summarize local changes |
| `git diff` | Review local changes |

If `orc.yaml` defines `repos[].worktree_setup`, use the setup command that
`orc next` prints instead of raw `git worktree add`.

After creating or using a worktree, update `STATE.yaml` through the process in
`ORC.md` so later stages know where repo work happened.

---

## Approval Required

Read `RULES.md` for the full approval policy. In general, ask before:

| Action                       | Why                             |
|------------------------------|---------------------------------|
| Writing to the ticket system | Visible to stakeholders         |
| Opening or merging PRs       | Affects shared branches         |
| Triggering CI/CD             | May affect shared environments  |
| Posting external comments    | Hard to retract                 |
