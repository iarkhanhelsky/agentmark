package comments

import (
	"fmt"
	"os"
	"strings"

	"agentmark/gateway"
)

const agentIntroMaxLen = 40

const agentIntroHelp = `with --role agent, the first line of --body must identify the source of the message.
Main chat/session: the tool/CLI name (e.g. "Cursor", "Claude", "Codex").
Subagent task: the stable subagent identifier prefixed with @ (e.g. "@explore").
Limit: %d chars on a single line.`

// validateAgentBody enforces a minimal-shape rule for --role agent:
// the first line of the body must be a non-blank intro line of at most
// agentIntroMaxLen characters. The exact token is not constrained —
// agents just need to identify their source with a stable short name.
// No-op for other roles.
func validateAgentBody(role, body string) error {
	if role != "agent" {
		return nil
	}
	first, _, _ := strings.Cut(body, "\n")
	trimmed := strings.TrimSpace(first)
	if trimmed == "" {
		return fmt.Errorf(agentIntroHelp+"\nGot: empty first line.", agentIntroMaxLen)
	}
	if n := len([]rune(trimmed)); n > agentIntroMaxLen {
		return fmt.Errorf(agentIntroHelp+"\nGot: %d chars.", agentIntroMaxLen, n)
	}
	return nil
}

func filterCommentThreads(threads []gateway.CommentThread, openOnly, resolvedOnly, detachedOnly bool) []gateway.CommentThread {
	if !openOnly && !resolvedOnly && !detachedOnly {
		out := make([]gateway.CommentThread, len(threads))
		copy(out, threads)
		return out
	}
	var filtered []gateway.CommentThread
	for _, t := range threads {
		if openOnly && (t.Resolved || t.Detached) {
			continue
		}
		if resolvedOnly && !t.Resolved {
			continue
		}
		if detachedOnly && !t.Detached {
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered
}

// filterAwaitingAgentReply keeps threads whose last message is role "user" (typical queue for an agent reply).
func filterAwaitingAgentReply(threads []gateway.CommentThread) []gateway.CommentThread {
	out := make([]gateway.CommentThread, 0, len(threads))
	for _, t := range threads {
		n := len(t.Thread)
		if n == 0 {
			continue
		}
		if t.Thread[n-1].Role == "user" {
			out = append(out, t)
		}
	}
	return out
}

// listThreadsForMarkdown loads sidecar, re-anchors against markdown, persists
// via SaveThreads (which skips creating a new file when threads is empty), then
// returns filtered threads.
func listThreadsForMarkdown(abs string, openOnly, resolvedOnly, detachedOnly bool) ([]gateway.CommentThread, error) {
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	markdown := string(raw)
	threads, err := gateway.LoadThreads(abs)
	if err != nil {
		return nil, err
	}
	snap, _ := gateway.NewSnapshotStore(abs)
	threads = gateway.ReanchorThreadsWithStore(markdown, threads, snap)
	if err := gateway.SaveThreads(abs, threads); err != nil {
		return nil, err
	}
	return filterCommentThreads(threads, openOnly, resolvedOnly, detachedOnly), nil
}
