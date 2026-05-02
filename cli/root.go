package cli

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"agentmark/server"

	"github.com/spf13/cobra"
)

// Execute runs the root command (and exits the process on error via the caller).
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	var port string
	var noOpen bool

	use := "agentmark <path>"
	if len(os.Args) > 0 && os.Args[0] != "" {
		use = filepath.Base(os.Args[0]) + " <path>"
	}

	cmd := &cobra.Command{
		Use:   use,
		Short: "Local-first markdown review: preview, anchored comments, snapshot history",
		Long: `AgentMark — local-first markdown review: preview, anchored comment threads
(sidecar JSON), and file snapshot history.

Serve a markdown file or directory at http://127.0.0.1:<port>/ and watch for changes.

Comments are stored in .<filename>.comments.json next to the document.
Snapshot history is kept under the OS app data directory (see README).`,
		Example: `  agentmark ./README.md
  agentmark --port 8080 --no-open ./docs/`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			if !noOpen {
				go func() {
					time.Sleep(400 * time.Millisecond)
					openBrowser("http://127.0.0.1:" + port)
				}()
			}
			return server.Start(ctx, server.Config{FilePath: args[0], Port: port})
		},
	}

	cmd.Flags().SortFlags = false
	cmd.Flags().StringVar(&port, "port", "4173", "HTTP listen port for the review UI")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "do not open a browser tab")

	cmd.AddCommand(
		newSkillCommand(),
		newSnapshotCommand(),
		newCommentsCommand(),
	)

	cmd.InitDefaultHelpCmd()
	return cmd
}

func openBrowser(url string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	_ = c.Start()
}
