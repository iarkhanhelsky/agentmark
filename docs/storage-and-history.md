# Storage and snapshot history

This page describes where AgentMark keeps comment threads and file snapshots on disk.

## Comment threads (sidecar)

For each markdown file `doc.md`, threads are stored next to it as:

`.doc.md.comments.json`

Prefer the `agentmark comments` CLI (or the review UI) instead of editing this file by hand.

## Snapshot history

Snapshots are keyed by the **absolute path** of the markdown file (hashed). They live under the OS app data directory, not inside your git repo.

Typical locations:

- **macOS**: `~/Library/Application Support/agentmark/history/<sha1>/`
- **Linux**: `$XDG_DATA_HOME/agentmark/history/<sha1>/` or `~/.local/share/agentmark/history/<sha1>/`
- **Windows**: `%APPDATA%\agentmark\history\<sha1>\`

New snapshots are debounced (about 2 seconds) after the file changes on disk. In the **History** tab you can diff two versions and apply selected hunks back into the working file.
