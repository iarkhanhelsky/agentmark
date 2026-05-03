package comments

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agentmark/cli/internal/cmdcore"
	"agentmark/gateway"

	"github.com/spf13/cobra"
)

// NewListCommand returns the cobra command that lists comment threads for a markdown file or directory tree.
func NewListCommand() *cobra.Command {
	var resolvedOnly, detachedOnly, openOnly, awaitingAgent, jsonOut bool
	c := &cobra.Command{
		Use:   "list <file.md|dir>",
		Short: "List comment threads for a markdown file or all .md files under a directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(args[0], listFlags{
				resolvedOnly:  resolvedOnly,
				detachedOnly:  detachedOnly,
				openOnly:      openOnly,
				awaitingAgent: awaitingAgent,
				jsonOut:       jsonOut,
			})
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

type listFlags struct {
	resolvedOnly, detachedOnly, openOnly, awaitingAgent, jsonOut bool
}

func runList(pathArg string, f listFlags) error {
	_ = f.jsonOut // output is always JSON; flag is kept for ergonomics.
	abs, isDir, err := cmdcore.AbsExistingPath(pathArg)
	if err != nil {
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	applyAwaiting := func(threads []gateway.CommentThread) []gateway.CommentThread {
		if !f.awaitingAgent {
			return threads
		}
		return filterAwaitingAgentReply(threads)
	}
	if !isDir {
		filtered, err := listThreadsForMarkdown(abs, f.openOnly, f.resolvedOnly, f.detachedOnly, true)
		if err != nil {
			cmdcore.WriteError(err)
			return cmdcore.ErrAlreadyReported
		}
		filtered = applyAwaiting(filtered)
		if err := enc.Encode(map[string]any{"file": abs, "threads": filtered}); err != nil {
			cmdcore.WriteError(err)
			return cmdcore.ErrAlreadyReported
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
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	sort.Strings(mdPaths)
	files := make([]map[string]any, 0)
	for _, p := range mdPaths {
		filtered, err := listThreadsForMarkdown(p, f.openOnly, f.resolvedOnly, f.detachedOnly, false)
		if err != nil {
			cmdcore.WriteError(err)
			return cmdcore.ErrAlreadyReported
		}
		filtered = applyAwaiting(filtered)
		if len(filtered) == 0 {
			continue
		}
		files = append(files, map[string]any{"file": p, "threads": filtered})
	}
	if err := enc.Encode(map[string]any{"root": abs, "files": files}); err != nil {
		cmdcore.WriteError(err)
		return cmdcore.ErrAlreadyReported
	}
	return nil
}
