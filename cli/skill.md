---
name: agentmark
description: >-
  Interact with AgentMark markdown review: comment threads (sidecar JSON) and
  named document snapshots via the agentmark CLI. Use when reading, writing, or
  resolving review comments on .md files, listing history, diffing versions, or
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
```

**List threads for every `*.md` under a directory** (recursive walk). JSON shape:
`{"root":"<abs dir>","files":[{"file":"<abs path>.md","threads":[...]}, ...]}` — only files with at least one matching thread after filters. Re-anchors each file; writes sidecars only when a sidecar already exists or that file has threads (does not create empty sidecars for uncommented markdown).

```bash
agentmark comments list .
agentmark comments list ./docs --open
```

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

**Resolve / reopen**:

```bash
agentmark comments resolve ./doc.md --thread <id>
agentmark comments unresolve ./doc.md --thread <id>
```

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

1. `agentmark comments list ./doc.md --open` — pick thread ids.
2. Edit the markdown in your editor or patch tool.
3. `agentmark comments reply ./doc.md --thread <id> --body "..." --role agent`
4. `agentmark comments resolve ./doc.md --thread <id>` when done.
5. `agentmark snapshot save ./doc.md --label "checkpoint"` before large edits.

## Install this skill

```bash
agentmark skill --install-cursor
agentmark skill --install-claude
```
