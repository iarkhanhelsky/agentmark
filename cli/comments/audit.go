package comments

import (
	"encoding/json"
	"fmt"
	"os"

	"agentmark/cli/internal/cmdcore"

	"github.com/spf13/cobra"
)

// NewAuditCommand returns the cobra command that reports detached unresolved threads and optional strict failure.
func NewAuditCommand() *cobra.Command {
	var strict bool
	c := &cobra.Command{
		Use:   "audit <file.md>",
		Short: "Warn if detached unresolved threads remain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAudit(args[0], strict)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.Flags().SortFlags = false
	c.Flags().BoolVar(&strict, "strict", false, "exit non-zero if detached unresolved threads exist")
	return c
}

func runAudit(pathArg string, strict bool) error {
	abs, err := cmdcore.AbsMarkdown(pathArg)
	if err != nil {
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	threads, err := listThreadsForMarkdown(abs, false, false, false)
	if err != nil {
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
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
			cmdcore.WriteError(fmt.Errorf("detached unresolved threads found"))
			return cmdcore.ErrAlreadyReported
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
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	return nil
}
