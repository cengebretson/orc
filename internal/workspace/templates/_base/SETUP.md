# Workspace Setup

Run this file with each agent you plan to use in this workspace:

```
claude "Read SETUP.md and follow the setup instructions"
codex  "Read SETUP.md and follow the setup instructions"
```

The shared sections only need to be completed once — whichever agent runs first
handles them. Each agent then completes its own section. Check the Status block
to see what still needs to be done before starting.

If this workspace was created with the default pack, the starter workflow,
stages, and workers already exist under `orc.yaml`, `stages/default/`, and
`workers/default/`. Setup should customize those resources or add namespaced
custom resources; it should not create root-level worker files.

---

## Status

shared: pending
claude: pending
codex:  pending

<!-- orc doctor checks these lines — do not remove them. -->
<!-- Change each to "complete" when that section is finished. -->

---

## Instructions for the Agent

1. Read the Status block above.
2. Skim `orc.yaml`, `ROUTER.md`, and `TOOLS.md` so you know what you are filling in.
   If packs are installed, run `orc pack list` to see what the workspace already has.
3. If `shared: pending` — complete the Shared sections first, then mark `shared: complete`.
4. If `shared: complete` — skip to your own agent section (Claude or Codex).
5. Complete your agent section and mark it `complete` in the Status block.
6. Print a summary of every file you created or updated.

Do not re-run sections already marked complete.

**Ask one section's questions at a time, batched.** Each section below lists all
of its questions together — ask them in a single message, wait for the user's
answers, then make every file edit for that section before moving to the next.
Do not drip questions out one at a time across many turns.

---

## Shared Section 1: Ticket System

The ticket system is described in two files, each owning a different concern.
Fill each field in its designated file and do not duplicate values between them —
`ROUTER.md` is the source of truth for *how to retrieve* a ticket.

**Ask the user (all at once):**
> 1. What system do you use for tickets or stories?
>    (Jira / GitHub Issues / Linear / local markdown files / none)
> 2. The exact command to retrieve a ticket by ID — or say "propose one" and I'll
>    suggest one based on your system for you to confirm.
> 3. Any authentication requirements? (env var, API key location)
> 4. Project / team keys?
> 5. Ticket URL format?
> 6. The MCP server name you gave this system, if any?

**Update `ROUTER.md` (retrieval — the source of truth):**
- The exact command to retrieve a ticket by ID. **Do not guess this.** If you
  don't already know the user's exact command, propose one and ask them to
  confirm or correct it before writing it (e.g. for GitHub Issues you can
  propose `gh issue view <id>`; for Jira/Linear/custom setups, ask).
- Any authentication requirements (env var, API key location) — ask the user

**Update `TOOLS.md` (identity and access):**
- System name
- Project / team keys
- Ticket URL format
- The MCP server name the user gave this system, if any (MCP config itself lives
  at the user level — `~/.claude/mcp.json` or the Codex equivalent — record only
  the name here). Ask the user for it.

---

## Shared Section 2: Source Control

**Ask the user (all at once):**
> 1. What source control system do you use?
>    (GitHub / GitLab / Bitbucket / other / none)
> 2. Org / repo? (e.g. myorg/myrepo)
> 3. Default branch? (e.g. main)
> 4. PR target branch? (e.g. main, or a release branch)
> 5. The MCP server name you gave source control, if any?

**Update `TOOLS.md`:**
- Fill in the platform name and the four fields above in the Source Control section
- Note: MCP servers are configured at the user level, not per-workspace —
  record only the name

---

## Shared Section 3: Repos and Routes

Repos live wherever they are on the filesystem. Worktrees for ticket work are
always created inside this workspace under `worktrees/`.

**Ask the user (all at once):**
> How many code repositories does this workspace orchestrate, and for each one:
> 1. Short name (e.g. "my-app", "qa-suite")
> 2. Full path on the filesystem (e.g. /Users/me/projects/my-app)
> 3. Purpose (one line)
> 4. Optional worktree setup command, if this repo needs more than raw
>    `git worktree add` (leave blank otherwise)
> 5. Optional repo hints agents should see before editing that repo

**Then update `orc.yaml`:**
- Replace the example entry under `repos:` with the real repos (name, path, purpose)
- If a repo needs custom worktree setup, add `worktree_setup`. Supported
  placeholders are `{{branch}}`, `{{worktree_path}}`, `{{repo_path}}`,
  `{{repo_name}}`, `{{ticket}}`, `{{slug}}`, and `{{workspace}}`. Include
  `{{worktree_path}}` so the command creates the checkout where orc expects it.
- If a repo has local conventions agents should not miss, add short
  `agent_hints` entries. Keep them generic and durable; stage-specific tasks
  belong in stage or worker files.

**And update `ROUTER.md`:**
- Fill in the worktree section with either the repo-specific setup rule or the
  `git worktree add` fallback for this workspace.

---

## Shared Section 4: Workflow and Workers

This section makes the workflow match the user's process and ensures every
`worker:` id referenced in `orc.yaml` resolves to a real worker file.

**Review the workflow with the user:**
- Show them the `workflows:` block in `orc.yaml` (the default flow is
  `default:intake → default:develop → default:pr-open → default:qa-automation`,
  with review/repair loops through `default:code-review` and `default:pr-repair`).
- If they are using packs, run `orc pack list` or `orc pack show <pack>` to show
  what each installed pack provides.
- Explain that `aliases:` may provide friendly names like `develop`, but files
  still live at namespaced paths such as `stages/default/develop.md`.
- Ask whether these stages and their order match how they work. Add, remove, or
  reorder stages as needed. Each stage references a worker by `id`.

**Find the required worker ids:**
- Run `orc doctor`. It reports any stage whose `worker:` id has no matching file —
  that is your checklist of workers that must exist.

**Make every worker id resolve** (engine and model are assigned later, in the
Claude / Codex sections):
- If you installed a pack (for example, the default pack from `orc init` or a
  later `orc pack install`), `workers/<pack>/`
  already contains its persona workers — `fred`, `bob`, `zach`, `brian`, and
  others — edit those rather than creating new ones.
- If `workers/` only has `_template.md` (e.g. `orc init --skip-default-pack`), copy it
  once per `worker:` id in `orc.yaml` using the namespaced path format, such as
  `workers/custom/chris.md` for worker ID `custom:chris`.
- Do not create root-level files such as `workers/chris.md`; runnable workers
  must use `workers/<namespace>/<name>.md` with a matching `<namespace>:<name>`
  frontmatter `id`.

---

## Shared Section 5: Team Conventions and Approval Policy

**Ask the user (all at once):**
> 1. Any team conventions agents should follow? (PR size, commit message style,
>    review norms, branch naming, anything else)
> 2. `RULES.md` requires human approval for opening PRs, triggering CI, writing to
>    the ticket system, and posting external comments. Do those defaults match your
>    team's policy, or should any be loosened or tightened?

**Then:**
- Record the conventions under the `## Team Conventions` heading at the bottom of
  `AGENTS.md`
- Adjust `RULES.md` if the user wanted any approval defaults changed

---

## Claude Section

**Ask the user (all at once):**
> 1. Do you want to use Claude in this workspace? (yes / no / already configured)
> 2. If yes — which Claude model should Claude-run workers use?
>    (claude-opus-4-8 / claude-sonnet-4-6 / claude-haiku-4-5-20251001)
> 3. Which worker roles should Claude run, or should the existing worker engine
>    assignments stay as-is?
> 4. Which MCP servers from ~/.claude/mcp.json should workers use? (names only —
>    they are already installed at the user level)

- If **no** — mark `claude: complete` and skip this section.
- If **already configured** — verify each Claude worker has `engine: claude` and a
  valid `model:`, confirm the `TOOLS.md` Claude MCP line is filled in, then mark
  `claude: complete`.

**Then:**
- For each worker you want Claude to run, set `engine: claude` and
  `model: <chosen model>` in its frontmatter. A worker runs on exactly one
  engine — assign each role to Claude or Codex, not both.
- Update `TOOLS.md` — in the **MCP Servers** section, fill in the **Claude** line
  with the server names the user provided.
- Run `orc doctor` and confirm no `orc.yaml` stage reports a missing worker.

---

## Codex Section

**Ask the user (all at once):**
> 1. Do you want to use Codex in this workspace? (yes / no / already configured)
> 2. If yes — which Codex model should Codex-run workers use? (or default)
> 3. Which worker roles should Codex run, or should the existing worker engine
>    assignments stay as-is?
> 4. Which MCP servers or tools configured for Codex should workers use? (names only)

- If **no** — mark `codex: complete` and skip this section.
- If **already configured** — verify each Codex worker has `engine: codex` and a
  valid `model:` (or a deliberate default), confirm the `TOOLS.md` Codex MCP line
  is filled in, then mark `codex: complete`.

**Then:**
- For each worker you want Codex to run, set `engine: codex` and
  `model: <chosen model or omit for default>` in its frontmatter. A worker runs
  on exactly one engine — assign each role to Claude or Codex, not both.
- Update `TOOLS.md` — in the **MCP Servers** section, fill in the **Codex** line
  with any tools or server names the user provided.
- Run `orc doctor` and confirm no `orc.yaml` stage reports a missing worker.

---

## Final Step

When your section is complete:
1. Update the Status block at the top — mark `shared: complete` if you completed it,
   and mark `claude: complete` or `codex: complete` for your agent section
2. Run `orc doctor` yourself and read the output. If it reports any problems
   (missing workers, unresolved files, an incomplete SETUP), fix them and run it
   again. Do not declare setup done until `orc doctor` is clean — then show the
   user the result.
3. Once at least one engine has runnable workers, tell the user they can now run
   `orc work <ticket>`
