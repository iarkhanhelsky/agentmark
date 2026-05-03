package comments

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"agentmark/cli/internal/cmdcore"
	"agentmark/gateway"

	"github.com/spf13/cobra"
)

// NewReplyCommand returns the cobra command that appends a message to an existing comment thread.
func NewReplyCommand() *cobra.Command {
	var threadID, body, role string
	c := &cobra.Command{
		Use:   "reply <file.md>",
		Short: "Append a message to an existing thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReply(args[0], threadID, body, role)
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

func runReply(pathArg, threadID, body, role string) error {
	if threadID == "" || strings.TrimSpace(body) == "" {
		cmdcore.WriteError(fmt.Errorf("--thread and --body are required"))
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
	threads, err := gateway.LoadThreads(abs)
	if err != nil {
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	found := false
	for i := range threads {
		if threads[i].ID == threadID {
			threads[i].Thread = append(threads[i].Thread, gateway.CommentMessage{
				Role: role, Body: body, TS: time.Now().UnixMilli(),
			})
			found = true
			break
		}
	}
	if !found {
		cmdcore.WriteError(fmt.Errorf("thread not found"))
		return cmdcore.ErrAlreadyReported
	}
	snap, _ := gateway.NewSnapshotStore(abs)
	threads = gateway.ReanchorThreadsWithStore(markdown, threads, snap)
	if err := gateway.SaveThreads(abs, threads); err != nil {
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]any{"ok": true, "threads": threads}); err != nil {
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	return nil
}
