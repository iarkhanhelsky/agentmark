# Agent instructions (AgentMark)

## Integration tests are mandatory for behavior changes

When a change modifies user-visible behavior, command/API output shape, or persistent side effects, update integration tests in the same change.

- CLI behavior changes (command args/validation, stdout/stderr JSON, sidecar writes) must add or update coverage under `cli/*_integration_test.go`.
- Serve/API behavior changes (startup/shutdown flow, handler responses, path validation, project/file selection) must add or update coverage under `server/*_integration_test.go`.
- Do not merge behavior changes without corresponding integration test updates; if no integration test is applicable, document why in the PR notes.
