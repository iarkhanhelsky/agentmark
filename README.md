# AgentMark

Local-first markdown **review** tool: read-only preview, anchored comment threads (sidecar JSON), and a **time machine** of file snapshots stored under your OS app data directory (outside the repo, so agents working in the tree cannot clobber history).

Agent invocation stays in your IDE (Cursor, Zed, VSCode). Use **Copy review context** to paste open threads into chat.

## Build

```bash
go build -o agentmark .
```

## Run

```bash
./agentmark ./path/to/doc.md
# optional: --port 4173 --no-open
```

Opens `http://127.0.0.1:4173` (default) and watches the markdown file. Comments live in `.<filename>.comments.json` next to the document.

## Snapshots

History path (per absolute file path, hashed):

- macOS: `~/Library/Application Support/agentmark/history/<sha1>/`
- Linux: `$XDG_DATA_HOME/agentmark/history/<sha1>/` or `~/.local/share/agentmark/history/<sha1>/`
- Windows: `%APPDATA%\agentmark\history\<sha1>\`

New snapshots are debounced (~2s) after each file change. The **History** tab diffs two snapshots and can **apply selected hunks** to the current file.

## Stack

- Go 1.22+ (`net/http`, `chi`, `fsnotify`, `gorilla/websocket`)
- Alpine.js + `marked` (CDN) in embedded static assets
