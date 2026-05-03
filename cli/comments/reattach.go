package comments

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"agentmark/cli/internal/cmdcore"
	"agentmark/server"

	"github.com/spf13/cobra"
)

// NewReattachCommand returns the cobra command that moves an existing thread to new anchor text in the document.
func NewReattachCommand() *cobra.Command {
	var threadID, anchor string
	c := &cobra.Command{
		Use:   "reattach <file.md>",
		Aliases: []string{
			"reattach-apply",
		},
		Short: "Move an existing thread to new anchor text (substring of the document)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReattach(args[0], threadID, anchor)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.Flags().SortFlags = false
	c.Flags().StringVar(&threadID, "thread", "", "existing thread id")
	c.Flags().StringVar(&anchor, "anchor", "", "new anchor substring (must appear in the file)")
	return c
}

func runReattach(pathArg, threadID, anchor string) error {
	if threadID == "" || strings.TrimSpace(anchor) == "" {
		cmdcore.WriteError(fmt.Errorf("--thread and --anchor are required"))
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
		cmdcore.WriteError(fmt.Errorf("thread not found"))
		return cmdcore.ErrAlreadyReported
	}
	snap, _ := server.NewSnapshotStore(abs)
	threads = server.ReanchorThreadsWithStore(markdown, threads, snap)
	if err := server.SaveThreads(abs, threads); err != nil {
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
