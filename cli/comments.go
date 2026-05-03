package cli

import (
	"agentmark/cli/comments"

	"github.com/spf13/cobra"
)

// newCommentsCommand returns the parent cobra command for anchored comment sidecar operations
// (list, add, reattach, reply, resolve, unresolve, audit).
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
		comments.NewListCommand(),
		comments.NewAddCommand(),
		comments.NewReattachCommand(),
		comments.NewReplyCommand(),
		comments.NewResolveCommand(),
		comments.NewUnresolveCommand(),
		comments.NewAuditCommand(),
	)
	return cmd
}
