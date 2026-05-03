<h1 align="left"><img src="docs/readme-wordmark.svg"  height="96" alt="AgentMark" /></h1>

**Motivation.** When you and an agent iterate on markdown, you need feedback tied to exact passages, not scattered chat context. AgentMark gives you anchored threads so humans can steer edits and agents can respond precisely.

**Core principles.**
- **System of record, not runner.** AgentMark owns review data; execution loops (daemon/chat/session orchestration) stay outside.
- **LLM-agnostic workflow.** Teams can use any LLM tool to work through content while keeping comments, anchors, and history in one place.
- **Explicit state ownership.** Server truth: document, threads, timestamps, detached/resolved state. Client truth: personal read/unread cursors in browser storage. External truth: automation lifecycle health.

**History without git noise.** Drafts change constantly; you do not want every iteration as a commit. AgentMark records automatic snapshots of the file (outside the repo) so you can step through versions, diff, and recover wording without treating git as a scratch pad.

## Get started

1. **Install** — release packages are TBD. Until then, build from source (see [Development](#development)) or use the repo’s `./wagentmark` wrapper (runs `go run .`).
2. **Run** — point AgentMark at a markdown file. This starts the local review server and opens your browser.

   ```bash
   ./wagentmark ./path/to/doc.md
   ```

   Optional: `--port 4173` (default), `--no-open` to skip opening a browser tab.
   Discover flags and subcommands with `./wagentmark --help`.

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
# Default policy: humans resolve threads unless they explicitly ask an agent to do it.
./wagentmark comments resolve ./README.md --thread <id>

# After an agent edit, list detached threads and re-point one at new text (same thread id)
./wagentmark comments list ./README.md --detached
./wagentmark comments reattach ./README.md --thread <id> --anchor "exact substring from file"

# Save a named snapshot
./wagentmark snapshot save ./README.md --label "after polish pass"

# Combine optional flags (for remote/headless usage)
./wagentmark --port 8080 --no-open ./docs/
```

## Where data lives

AgentMark stores review state outside your document content:
- Comment threads in sidecar files next to markdown docs.
- Snapshot history in OS app data storage (outside your git repo).

See [docs/storage-and-history.md](docs/storage-and-history.md) for exact paths, formats, and retention details.

## Development

```bash
go build -o agentmark .
./agentmark ./path/to/doc.md
```

In this repository, `./wagentmark` is a thin wrapper around `go run .` for the same CLI.

## Stack

- Go 1.22+ (`net/http`, `chi`, `fsnotify`, `gorilla/websocket`)
- Alpine.js + `marked` (CDN) in embedded static assets
  - CDN assets are version-pinned; restricted/offline environments may require mirroring or local vendoring.
