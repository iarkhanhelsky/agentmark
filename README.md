# AgentMark

**Motivation.** When you and an agent iterate on markdown, you need a quick way to leave feedback that stays tied to the text—not another long paste into chat. AgentMark gives you anchored comment threads on the document so humans can steer and agents can respond with concrete edits.

**Collaboration.** You work in the review UI (or your IDE integration): add comments on passages, keep threads open until the wording matches intent, then resolve them. Agents can follow the same threads from **Copy review context** or via the `comments` CLI against the file on disk.

**History without git noise.** Drafts change constantly; you do not want every iteration as a commit. AgentMark records automatic snapshots of the file (outside the repo) so you can step through versions, diff, and recover wording without treating git as a scratch pad.

## Get started

1. **Install** — release packages are TBD. Until then, build from source (see [Development](#development)) or use the repo’s `./wagentmark` wrapper (runs `go run .`).
2. **Run** — point AgentMark at a markdown file. This starts the local review server and opens your browser.

   ```bash
   ./wagentmark ./path/to/doc.md
   ```

   Optional: `--port 4173` (default), `--no-open` to skip opening a browser tab.

3. **Use the UI** — read the preview, add comments anchored to the text, browse **History** to compare snapshots or pull back earlier wording.

For agent-assisted workflows, paste **Copy review context** into your IDE chat, or use the CLI examples below.

## CLI workflow

The first argument to the binary is always the markdown file or directory to serve; there is no separate `serve` subcommand.

```bash
# Open the review UI for a file
./wagentmark ./README.md

# List unresolved threads that still anchor in the file
./wagentmark comments list ./README.md --open

# Add a thread anchored to exact text in the file
./wagentmark comments add ./README.md --anchor "exact substring" --body "Please clarify this section"

# Reply and resolve
./wagentmark comments reply ./README.md --thread <id> --body "Updated in latest edit" --role agent
./wagentmark comments resolve ./README.md --thread <id>

# After an agent edit, list detached threads and re-point one at new text (same thread id)
./wagentmark comments list ./README.md --detached
./wagentmark comments reattach ./README.md --thread <id> --anchor "exact substring from file"

# Save a named snapshot
./wagentmark snapshot save ./README.md --label "after polish pass"
```

## IDE workflow

Run AgentMark from your IDE (Cursor, Zed, VSCode), then use **Copy review context** to paste open review threads into chat. Each thread line includes a stable **id** so agents can preserve or re-attach anchors after edits. Threads that fall off the document appear under **detached** in the UI and in a `[DETACHED]` section in the copied context; use **Re-attach** (select new text) or `comments reattach` to link them again.

## Where data lives

Comment sidecars and snapshot paths are described in [docs/storage-and-history.md](docs/storage-and-history.md).

## Development

```bash
go build -o agentmark .
./agentmark ./path/to/doc.md
```

In this repository, `./wagentmark` is a thin wrapper around `go run .` for the same CLI.

## Stack

- Go 1.22+ (`net/http`, `chi`, `fsnotify`, `gorilla/websocket`)
- Alpine.js + `marked` (CDN) in embedded static assets
