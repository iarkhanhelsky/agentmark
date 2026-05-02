# Agent instructions (AgentMark)

## Comment threads (`role: agent`)

When adding or replying as **agent** (including via CLI `comments add` / `comments reply` with default `--role agent`), start the message body with one short line, then the substantive text:

- Main agent only: `Claude, Cursor`
- Named subagent task: `Claude, Cursor, @SubagentName` (use the stable subagent identifier, e.g. `@explore`)

Put a blank line after the intro line before the rest of the message when it helps readability.

Human-authored messages (`role: user`) do not use this prefix.

## Integration tests are mandatory for behavior changes

When a change modifies user-visible behavior, command/API output shape, or persistent side effects, update integration tests in the same change.

- CLI behavior changes (command args/validation, stdout/stderr JSON, sidecar writes) must add or update coverage under `cli/*_integration_test.go`.
- Serve/API behavior changes (startup/shutdown flow, handler responses, path validation, project/file selection) must add or update coverage under `server/*_integration_test.go`.
- Do not merge behavior changes without corresponding integration test updates; if no integration test is applicable, document why in the PR notes.
