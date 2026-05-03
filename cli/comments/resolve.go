package comments

import (
	"encoding/json"
	"fmt"
	"os"

	"agentmark/cli/internal/cmdcore"
	"agentmark/server"

	"github.com/spf13/cobra"
)

// NewResolveCommand returns the cobra command that marks a thread as resolved.
func NewResolveCommand() *cobra.Command {
	return newResolveCommand("resolve", true)
}

// NewUnresolveCommand returns the cobra command that marks a thread as unresolved.
func NewUnresolveCommand() *cobra.Command {
	return newResolveCommand("unresolve", false)
}

func newResolveCommand(use string, resolved bool) *cobra.Command {
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
			return runResolve(args[0], threadID, resolved)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.Flags().SortFlags = false
	c.Flags().StringVar(&threadID, "thread", "", "thread id")
	return c
}

func runResolve(pathArg, threadID string, resolved bool) error {
	if threadID == "" {
		cmdcore.WriteError(fmt.Errorf("--thread is required"))
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
	for i := range threads {
		if threads[i].ID == threadID {
			threads[i].Resolved = resolved
			found = true
			break
		}
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
