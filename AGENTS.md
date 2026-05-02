# Agent instructions (AgentMark)

## Comment threads (`role: agent`)

When adding or replying as **agent** (including via CLI `comments add` / `comments reply` with default `--role agent`), start the message body with one short line, then the substantive text:

- Main agent only: `Claude, Cursor`
- Named subagent task: `Claude, Cursor, @SubagentName` (use the stable subagent identifier, e.g. `@explore`)

Put a blank line after the intro line before the rest of the message when it helps readability.

Human-authored messages (`role: user`) do not use this prefix.
