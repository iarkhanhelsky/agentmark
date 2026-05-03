package comments

import (
	"fmt"
	"os"
	"strings"

	"agentmark/server"
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

func filterCommentThreads(threads []server.CommentThread, openOnly, resolvedOnly, detachedOnly bool) []server.CommentThread {
	if !openOnly && !resolvedOnly && !detachedOnly {
		out := make([]server.CommentThread, len(threads))
		copy(out, threads)
		return out
	}
	var filtered []server.CommentThread
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
func filterAwaitingAgentReply(threads []server.CommentThread) []server.CommentThread {
	out := make([]server.CommentThread, 0, len(threads))
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

// listThreadsForMarkdown loads sidecar, re-anchors against markdown, optionally persists, returns filtered threads.
func listThreadsForMarkdown(abs string, openOnly, resolvedOnly, detachedOnly, alwaysSave bool) ([]server.CommentThread, error) {
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	markdown := string(raw)
	threads, err := server.LoadThreads(abs)
	if err != nil {
		return nil, err
	}
	snap, _ := server.NewSnapshotStore(abs)
	threads = server.ReanchorThreadsWithStore(markdown, threads, snap)
	sidecar := server.CommentsPathFor(abs)
	_, statErr := os.Stat(sidecar)
	hadSidecar := statErr == nil
	if alwaysSave || len(threads) > 0 || hadSidecar {
		if err := server.SaveThreads(abs, threads); err != nil {
			return nil, err
		}
	}
	return filterCommentThreads(threads, openOnly, resolvedOnly, detachedOnly), nil
}
