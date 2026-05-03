---
name: agentmark
description: >-
  Interact with AgentMark markdown review: comment threads (sidecar JSON) and
  named document snapshots via the agentmark CLI. Use when reading, writing, or
  replying to review comments on .md files, listing history, diffing versions, or
  saving labeled checkpoints.
---

# AgentMark CLI for agents

AgentMark stores **comment threads** next to each markdown file in `.<basename>.comments.json`.
**Snapshots** (version history) live under the OS app data directory, keyed by the absolute path of the file.

Always prefer **`agentmark comments`** and **`agentmark snapshot`** instead of editing raw sidecar JSON.

## Data model

- **Thread**: `id`, `anchorText`, optional `anchor` (offsets + prefix/suffix), `detached` (anchor not found in doc), `resolved`, `thread` (messages).
- **Message**: `role` (`user` | `agent`), `body`, `ts` (ms).
- **Snapshot**: `id` is UTC timestamp string; optional `label` and `auto` in metadata.

## Output conventions

- Successful JSON commands print **one JSON object** to **stdout** (pretty-printed).
- Errors: JSON `{"error":"..."}` on **stderr** and **non-zero exit code**.

## Open the review UI

The root command takes the markdown file (or directory) path; there is no `serve` subcommand.

```bash
agentmark ./docs/README.md
```

## Comments

**List threads** (re-anchors against file on disk; updates sidecar):

```bash
agentmark comments list ./doc.md
agentmark comments list ./doc.md --open        # unresolved && !detached
agentmark comments list ./doc.md --resolved    # only resolved
agentmark comments list ./doc.md --detached    # only detached
agentmark comments list ./doc.md --awaiting-agent  # last message is user (reply queue)
```

**List threads for every `*.md` under a directory** (recursive walk). JSON shape:
`{"root":"<abs dir>","files":[{"file":"<abs path>.md","threads":[...]}, ...]}` — only files with at least one matching thread after filters. Re-anchors each file; writes sidecars only when a sidecar already exists or that file has threads (does not create empty sidecars for uncommented markdown).

```bash
agentmark comments list .
agentmark comments list ./docs --open
agentmark comments list . --open --awaiting-agent   # actionable open threads waiting on an agent
```

Make `comments list` your default first step before substantive edits so thread context is loaded before you decide what to rewrite.

### Proactive awareness (Web UI comments)

LLM sessions do not automatically poll the repo. Something in **your** workflow must surface new sidecar activity:

- **Standing instruction:** Add to AGENTS.md / Cursor rules (or equivalent) that the agent should run `agentmark comments list <paths> --open --awaiting-agent` at the start of a task or after human review time.
- **Filesystem:** Watch `*.comments.json` with `entr`, `fswatch`, or a short IDE task; on change, run `comments list` and paste or pipe the JSON summary into the agent.
- **Hooks:** Cursor/Claude hooks can call a tiny script on sidecar save—the script should stay vendor-agnostic (invoke `agentmark`, print stdout). Same pattern works in other IDEs with file-watch tasks.
- **Future product:** A `comments watch` subcommand (newline-delimited JSON events) would reduce glue code; until then, polling + watchers are the portable contract (see roadmap P0).

### Message body format (`--role agent`)

When you post with `--role agent` (the default), the first line must identify **where the message came from** so threads stay attributable. Use a single short token:

- **From the main chat/session** — the tool/CLI name, e.g. `Cursor`, `Claude`, `Codex`.
- **From a named subagent task** — the stable subagent identifier prefixed with `@`, e.g. `@explore`, `@code-reviewer`.

The CLI enforces a minimal shape only: the first line must be non-blank and at most 40 characters; the exact token is up to you. Put a blank line after the intro before substantive text when it helps readability. `--role user` is not validated.

**New thread** (substring `anchor` must appear in the file for a stable anchor):

```bash
agentmark comments add ./doc.md --anchor "exact substring" --body "..." [--role agent]
```

**Reply**:

```bash
agentmark comments reply ./doc.md --thread <id> --body "..." [--role agent]
```

**Resolve / reopen** (manual control):

```bash
agentmark comments resolve ./doc.md --thread <id>
agentmark comments unresolve ./doc.md --thread <id>
```

Agent behavior rule:
- Do **not** resolve threads as part of agent workflow. Resolution stays human-owned.
- Instead, surface open threads requiring agent action:
  - open thread where the latest message is from `user` (agent reply required)
  - open thread explicitly requesting an action from the agent

## Rewriting documents (agent-owned edits)

Tier-0 rule: when you rewrite content, preserve thread intent during the same task. Do not defer this to later cleanup.

1. Before rewriting, run:

```bash
agentmark comments list ./doc.md --open
```

Capture thread IDs in the region you are about to modify.

2. Perform the rewrite.

3. In the same task, declare disposition for each affected thread:
   - Reattach to new text:

```bash
agentmark comments reattach ./doc.md --thread <id> --anchor "new exact substring in updated doc"
```

   - And/or reply with outcome when context changed or a human should close it:

```bash
agentmark comments reply ./doc.md --thread <id> --body "Cursor

Updated section X to address Y; please resolve if this is now complete." --role agent
```

Never use `comments resolve` as an agent.

Closure check before finishing a rewrite task:

```bash
agentmark comments list ./doc.md --detached
```

If detachments remain, reattach what you can and explain any remaining cases in thread replies for human review.

Algorithm safety net: server-side re-anchoring helps when mappings are missed, but explicit agent reattach is preferred whenever you know the semantic mapping at rewrite time.

## Snapshots

**List** history for a file:

```bash
agentmark snapshot list ./doc.md
```

**Save current file as a named checkpoint**:

```bash
agentmark snapshot save ./doc.md --label "after editorial pass"
```

**Label an existing snapshot id**:

```bash
agentmark snapshot label ./doc.md --id <timestamp-id> --label "v1 review"
```

**Read snapshot body**:

```bash
agentmark snapshot read ./doc.md --id <id>
```

**Diff** snapshot `a` to working file (omit `--b`) or to another snapshot:

```bash
agentmark snapshot diff ./doc.md --a <id>
agentmark snapshot diff ./doc.md --a <id> --b <other-id>
```

## Workflow recipe

1. `agentmark comments list ./doc.md --open` — gather active thread context first.
2. Edit the markdown in your editor or patch tool.
3. If rewrites touched anchored content, run `comments reattach` per affected thread.
4. `agentmark comments reply ./doc.md --thread <id> --body "..." --role agent` with what changed.
5. `agentmark comments list ./doc.md --detached` and clear or explain detachments.
6. `agentmark snapshot save ./doc.md --label "checkpoint"` before or after large edits.

## Install this skill

```bash
agentmark skill --install-cursor
agentmark skill --install-claude
```
