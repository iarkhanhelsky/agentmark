package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agentmark/server"

	"github.com/spf13/cobra"
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

func newCommentsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comments",
		Short: "List and mutate anchored comment threads (sidecar JSON)",
		Long: `Operate on .<filename>.comments.json next to a markdown file (or list all
markdown under a directory). Output is JSON on stdout; errors are JSON on stderr.

For --role agent (default on add/reply), the first line of --body must identify
the source of the message:
  - Main chat/session: the tool/CLI name (e.g. "Cursor", "Claude", "Codex").
  - Subagent task: the stable subagent identifier prefixed with @ (e.g. "@explore").
Put a blank line after the intro before substantive text when it helps readability.

comments list --awaiting-agent narrows to threads whose last message is role user (typical agent reply queue).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newCommentsListCommand(),
		newCommentsAddCommand(),
		newCommentsReattachCommand(),
		newCommentsReplyCommand(),
		newCommentsResolveCommand("resolve", true),
		newCommentsResolveCommand("unresolve", false),
		newCommentsAuditCommand(),
	)
	return cmd
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

func newCommentsListCommand() *cobra.Command {
	var resolvedOnly, detachedOnly, openOnly, awaitingAgent, jsonOut bool
	c := &cobra.Command{
		Use:   "list <file.md|dir>",
		Short: "List comment threads for a markdown file or all .md files under a directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = jsonOut // output is always JSON; flag is kept for ergonomics.
			abs, isDir, err := AbsExistingPath(args[0])
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			applyAwaiting := func(threads []server.CommentThread) []server.CommentThread {
				if !awaitingAgent {
					return threads
				}
				return filterAwaitingAgentReply(threads)
			}
			if !isDir {
				filtered, err := listThreadsForMarkdown(abs, openOnly, resolvedOnly, detachedOnly, true)
				if err != nil {
					WriteError(err)
					return ErrAlreadyReported
				}
				filtered = applyAwaiting(filtered)
				if err := enc.Encode(map[string]any{"file": abs, "threads": filtered}); err != nil {
					WriteError(err)
					return ErrAlreadyReported
				}
				return nil
			}
			var mdPaths []string
			err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if d.IsDir() {
					return nil
				}
				if strings.EqualFold(filepath.Ext(path), ".md") {
					mdPaths = append(mdPaths, path)
				}
				return nil
			})
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			sort.Strings(mdPaths)
			files := make([]map[string]any, 0)
			for _, p := range mdPaths {
				filtered, err := listThreadsForMarkdown(p, openOnly, resolvedOnly, detachedOnly, false)
				if err != nil {
					WriteError(err)
					return ErrAlreadyReported
				}
				filtered = applyAwaiting(filtered)
				if len(filtered) == 0 {
					continue
				}
				files = append(files, map[string]any{"file": p, "threads": filtered})
			}
			if err := enc.Encode(map[string]any{"root": abs, "files": files}); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.Flags().SortFlags = false
	c.Flags().BoolVar(&resolvedOnly, "resolved", false, "only threads marked resolved")
	c.Flags().BoolVar(&detachedOnly, "detached", false, "only detached threads")
	c.Flags().BoolVar(&openOnly, "open", false, "only unresolved, non-detached threads")
	c.Flags().BoolVar(&awaitingAgent, "awaiting-agent", false, "only threads whose last message has role user (agent follow-up queue)")
	c.Flags().BoolVar(&jsonOut, "json", false, "output JSON (no-op; output is always JSON)")
	return c
}

func newCommentsAddCommand() *cobra.Command {
	var anchor, body, role string
	c := &cobra.Command{
		Use:   "add <file.md>",
		Short: "Add a new comment thread anchored to text in the document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(anchor) == "" || strings.TrimSpace(body) == "" {
				WriteError(fmt.Errorf("--anchor and --body are required"))
				return ErrAlreadyReported
			}
			if err := validateAgentBody(role, body); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			abs, err := AbsMarkdown(args[0])
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			raw, err := os.ReadFile(abs)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			markdown := string(raw)
			threads, err := server.LoadThreads(abs)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			id := fmt.Sprintf("cli-%d", time.Now().UnixNano())
			t := server.CommentThread{ID: id, AnchorText: strings.TrimSpace(anchor), Thread: nil}
			hint := strings.Index(markdown, anchor)
			if hint < 0 {
				WriteError(fmt.Errorf("anchor text not found exactly in file; copy the exact markdown substring (including backticks/punctuation)"))
				return ErrAlreadyReported
			}
			end := hint + len(strings.TrimSpace(anchor))
			an := server.BuildAnchor(markdown, hint, end)
			t.Anchor = &an
			t.Thread = append(t.Thread, server.CommentMessage{
				Role: role, Body: body, TS: time.Now().UnixMilli(),
			})
			threads = append(threads, t)
			snap, _ := server.NewSnapshotStore(abs)
			threads = server.ReanchorThreadsWithStore(markdown, threads, snap)
			if err := server.SaveThreads(abs, threads); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			if err := enc.Encode(map[string]any{"ok": true, "threadId": id, "threads": threads}); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.Flags().SortFlags = false
	c.Flags().StringVar(&anchor, "anchor", "", "anchor text (substring of document)")
	c.Flags().StringVar(&body, "body", "", "message body; for --role agent, first line is the source token (e.g. \"Cursor\" or \"@explore\")")
	c.Flags().StringVar(&role, "role", "agent", "message role: user | agent (agent: see comments --help for first-line convention)")
	return c
}

func newCommentsReattachCommand() *cobra.Command {
	var threadID, anchor string
	c := &cobra.Command{
		Use:   "reattach <file.md>",
		Aliases: []string{
			"reattach-apply",
		},
		Short: "Move an existing thread to new anchor text (substring of the document)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if threadID == "" || strings.TrimSpace(anchor) == "" {
				WriteError(fmt.Errorf("--thread and --anchor are required"))
				return ErrAlreadyReported
			}
			abs, err := AbsMarkdown(args[0])
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			raw, err := os.ReadFile(abs)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			markdown := string(raw)
			threads, err := server.LoadThreads(abs)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			found := false
			at := strings.TrimSpace(anchor)
			for i := range threads {
				if threads[i].ID != threadID {
					continue
				}
				found = true
				threads[i].AnchorText = at
				hint := strings.Index(markdown, at)
				if hint >= 0 {
					end := hint + len(at)
					an := server.BuildAnchor(markdown, hint, end)
					threads[i].Anchor = &an
				} else {
					threads[i].Anchor = nil
				}
				break
			}
			if !found {
				WriteError(fmt.Errorf("thread not found"))
				return ErrAlreadyReported
			}
			snap, _ := server.NewSnapshotStore(abs)
			threads = server.ReanchorThreadsWithStore(markdown, threads, snap)
			if err := server.SaveThreads(abs, threads); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			if err := enc.Encode(map[string]any{"ok": true, "threads": threads}); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.Flags().SortFlags = false
	c.Flags().StringVar(&threadID, "thread", "", "existing thread id")
	c.Flags().StringVar(&anchor, "anchor", "", "new anchor substring (must appear in the file)")
	return c
}

func newCommentsAuditCommand() *cobra.Command {
	var strict bool
	c := &cobra.Command{
		Use:   "audit <file.md>",
		Short: "Warn if detached unresolved threads remain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			abs, err := AbsMarkdown(args[0])
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			threads, err := listThreadsForMarkdown(abs, false, false, false, true)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			detachedOpen := 0
			for _, t := range threads {
				if t.Detached && !t.Resolved {
					detachedOpen++
				}
			}
			if detachedOpen > 0 {
				_, _ = fmt.Fprintf(os.Stderr, "WARNING: %d detached unresolved thread(s) found in %s\n", detachedOpen, abs)
				_, _ = fmt.Fprintf(os.Stderr, "Run `agentmark comments list %s --detached` to inspect and reattach or reply.\n", abs)
				if strict {
					WriteError(fmt.Errorf("detached unresolved threads found"))
					return ErrAlreadyReported
				}
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			if err := enc.Encode(map[string]any{
				"ok":           true,
				"file":         abs,
				"detachedOpen": detachedOpen,
			}); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.Flags().SortFlags = false
	c.Flags().BoolVar(&strict, "strict", false, "exit non-zero if detached unresolved threads exist")
	return c
}

func newCommentsReplyCommand() *cobra.Command {
	var threadID, body, role string
	c := &cobra.Command{
		Use:   "reply <file.md>",
		Short: "Append a message to an existing thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if threadID == "" || strings.TrimSpace(body) == "" {
				WriteError(fmt.Errorf("--thread and --body are required"))
				return ErrAlreadyReported
			}
			if err := validateAgentBody(role, body); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			abs, err := AbsMarkdown(args[0])
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			raw, err := os.ReadFile(abs)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			markdown := string(raw)
			threads, err := server.LoadThreads(abs)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			found := false
			for i := range threads {
				if threads[i].ID == threadID {
					threads[i].Thread = append(threads[i].Thread, server.CommentMessage{
						Role: role, Body: body, TS: time.Now().UnixMilli(),
					})
					found = true
					break
				}
			}
			if !found {
				WriteError(fmt.Errorf("thread not found"))
				return ErrAlreadyReported
			}
			snap, _ := server.NewSnapshotStore(abs)
			threads = server.ReanchorThreadsWithStore(markdown, threads, snap)
			if err := server.SaveThreads(abs, threads); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			if err := enc.Encode(map[string]any{"ok": true, "threads": threads}); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.Flags().SortFlags = false
	c.Flags().StringVar(&threadID, "thread", "", "thread id")
	c.Flags().StringVar(&body, "body", "", "reply body; for --role agent, first line is the source token (e.g. \"Cursor\" or \"@explore\")")
	c.Flags().StringVar(&role, "role", "agent", "message role: user | agent (agent: see comments --help for first-line convention)")
	return c
}

func newCommentsResolveCommand(use string, resolved bool) *cobra.Command {
	var threadID string
	short := "Mark a thread as resolved"
	if !resolved {
		short = "Mark a thread as unresolved"
	}
	c := &cobra.Command{
		Use:   use + " <file.md>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if threadID == "" {
				WriteError(fmt.Errorf("--thread is required"))
				return ErrAlreadyReported
			}
			abs, err := AbsMarkdown(args[0])
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			raw, err := os.ReadFile(abs)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			markdown := string(raw)
			threads, err := server.LoadThreads(abs)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			found := false
			for i := range threads {
				if threads[i].ID == threadID {
					threads[i].Resolved = resolved
					found = true
					break
				}
			}
			if !found {
				WriteError(fmt.Errorf("thread not found"))
				return ErrAlreadyReported
			}
			snap, _ := server.NewSnapshotStore(abs)
			threads = server.ReanchorThreadsWithStore(markdown, threads, snap)
			if err := server.SaveThreads(abs, threads); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			if err := enc.Encode(map[string]any{"ok": true, "threads": threads}); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.Flags().SortFlags = false
	c.Flags().StringVar(&threadID, "thread", "", "thread id")
	return c
}
