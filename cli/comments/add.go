package comments

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"agentmark/cli/internal/cmdcore"
	"agentmark/server"

	"github.com/spf13/cobra"
)

// NewAddCommand returns the cobra command that adds a new anchored comment thread to a markdown file.
func NewAddCommand() *cobra.Command {
	var anchor, body, role string
	c := &cobra.Command{
		Use:   "add <file.md>",
		Short: "Add a new comment thread anchored to text in the document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(args[0], anchor, body, role)
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

func runAdd(pathArg, anchor, body, role string) error {
	if strings.TrimSpace(anchor) == "" || strings.TrimSpace(body) == "" {
		cmdcore.WriteError(fmt.Errorf("--anchor and --body are required"))
		return cmdcore.ErrAlreadyReported
	}
	if err := validateAgentBody(role, body); err != nil {
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	abs, err := cmdcore.AbsMarkdown(pathArg)
	if err != nil {
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	markdown := string(raw)
	threads, err := server.LoadThreads(abs)
	if err != nil {
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	id := fmt.Sprintf("cli-%d", time.Now().UnixNano())
	t := server.CommentThread{ID: id, AnchorText: strings.TrimSpace(anchor), Thread: nil}
	hint := strings.Index(markdown, anchor)
	if hint < 0 {
		cmdcore.WriteError(fmt.Errorf("anchor text not found exactly in file; copy the exact markdown substring (including backticks/punctuation)"))
		return cmdcore.ErrAlreadyReported
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
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]any{"ok": true, "threadId": id, "threads": threads}); err != nil {
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	return nil
}
