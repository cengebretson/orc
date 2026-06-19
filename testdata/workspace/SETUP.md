# Workspace Setup

Run this file with each agent you plan to use in this workspace:

```
claude "Read SETUP.md and follow the setup instructions"
codex  "Read SETUP.md and follow the setup instructions"
```

The shared sections only need to be completed once — whichever agent runs first
handles them. Each agent then completes its own section. Check the Status block
to see what still needs to be done before starting.

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
2. If `shared: pending` — complete the Shared sections first, then mark `shared: complete`.
3. If `shared: complete` — skip to your own agent section (Claude or Codex).
4. Complete your agent section and mark it `complete` in the Status block.
5. Print a summary of every file you created or updated.

Do not re-run sections already marked complete.

---

## Shared Section 1: Ticket System

**Ask the user:**
> What system do you use for tickets or stories?
> (1) Jira  (2) GitHub Issues  (3) Linear  (4) Local markdown files  (5) None / manual

**Update `ROUTER.md`:**
- Fill in the Ticket System section with the chosen system, project/team key,
  the exact command to retrieve a ticket by ID, and any auth requirements

**Also update `TOOLS.md`:**
- In the Ticket System section, fill in the system name and any required fields
- Note: MCP server config lives at the user level (~/.claude/mcp.json or equivalent)
  Ask the user what name they gave the MCP server (if any) and record it here

---

## Shared Section 2: Source Control

**Ask the user:**
> What source control system do you use?
> (1) GitHub  (2) GitLab  (3) Bitbucket  (4) Other / none

**Then update `TOOLS.md`:**
- In the Source Control section, fill in the platform name
- Ask the user what name they gave the source control MCP server (if any) and record it
- Note: MCP servers should be configured at the user level, not per-workspace

---

## Shared Section 3: Repos and Routes

Repos live wherever they are on the filesystem. Worktrees for ticket work are
always created inside this workspace under `worktrees/`.

**Ask the user:**
> How many code repositories does this workspace orchestrate?

**For each repo ask:**
> 1. Short name (e.g. "my-app", "qa-suite")
> 2. Full path on the filesystem (e.g. /Users/me/projects/my-app)
> 3. Purpose (one line)

**Then update `orc.yaml`:**
- Replace the example entry under `repos:` with the real repos (name, path, purpose)

**And update `ROUTER.md`:**
- Fill in the `git worktree add` example command with the actual workspace path

---

## Claude Section

**Ask the user:**
> Do you want to use Claude in this workspace? (yes / no / already configured)

If no — mark `claude: complete` and skip this section.

**Ask:**
> Which Claude model should the intake agent use?
> (1) claude-opus-4-7  (2) claude-sonnet-4-6  (3) claude-haiku-4-5-20251001

**Ask:**
> Do you have MCP servers configured in ~/.claude/mcp.json?
> List the names of any you want agents in this workspace to use.
> (These are already installed at the user level — we just need the names.)

**Then:**
- Create `workers/intake-agent-claude.md`:

```markdown
---
id: intake-agent-claude
name: Intake Agent (Claude)
engine: claude
model: <chosen model>
---

Fetches ticket context and populates the feature folder.
Reads ROUTER.md for ticket system details, then stages/default/intake.md for steps.
```

- Update `TOOLS.md` — in the Claude section, list the MCP server names the user provided

---

## Codex Section

**Ask the user:**
> Do you want to use Codex in this workspace? (yes / no / already configured)

If no — mark `codex: complete` and skip this section.

**Ask:**
> Which Codex model should the intake agent use? (press enter for default)

**Ask:**
> Do you have MCP servers or tools configured for Codex?
> List the names of any you want agents in this workspace to use.

**Then:**
- Create `workers/intake-agent-codex.md`:

```markdown
---
id: intake-agent-codex
name: Intake Agent (Codex)
engine: codex
model: <chosen model or omit for default>
---

Fetches ticket context and populates the feature folder.
Reads ROUTER.md for ticket system details, then stages/default/intake.md for steps.
```

- Update `TOOLS.md` — in the Codex section, list any tools or MCP servers the user provided

---

## Final Step

When your section is complete:
1. Update the Status block at the top — mark `shared: complete` if you completed it,
   and mark `claude: complete` or `codex: complete` for your agent section
2. Tell the user to run `orc doctor` to verify the workspace is ready
3. If both agents are configured, tell the user they can now run `orc work <ticket>`
