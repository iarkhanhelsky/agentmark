# Agent instructions (AgentMark)

## Integration tests are mandatory for behavior changes

When a change modifies user-visible behavior, command/API output shape, or persistent side effects, update integration tests in the same change.

- CLI behavior changes (command args/validation, stdout/stderr JSON, sidecar writes) must add or update coverage under `cli/*_integration_test.go`.
- Serve/API behavior changes (startup/shutdown flow, handler responses, path validation, project/file selection) must add or update coverage under `server/*_integration_test.go`.
- Do not merge behavior changes without corresponding integration test updates; if no integration test is applicable, document why in the PR notes.

## Cursor Cloud specific instructions

- **Language & tooling:** Pure Go 1.22+ project. No npm/node/docker needed.
- **Standard commands** are in `Makefile`: `make build`, `make test`, `make vet`, `make fmt`, `make run`. See the Makefile for details.
- **Running the server:** `./agentmark --no-open --port 4173 ./README.md` (or any `.md` file/directory). Use `--no-open` in headless environments to skip the browser-launch attempt.
- **Agent role messages:** When using `comments reply` or `comments add` with `--role agent`, the first line of `--body` must be a short identifier (e.g. `Cursor`), max 40 chars. Multi-line body uses `$'line1\nline2'` shell quoting.
- **Sidecar files:** Comment threads are stored as `.{filename}.comments.json` next to the markdown file. These are gitignored. Snapshot history is stored in the OS app-data directory (outside the repo).
- **Frontend:** Embedded static assets served from `server/static/`. Alpine.js and marked.js are loaded from CDN, so tests that render the full UI require network access.
