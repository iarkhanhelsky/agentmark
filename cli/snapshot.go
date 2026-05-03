package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"agentmark/gateway"

	"github.com/spf13/cobra"
)

func newSnapshotCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "List, save, and diff markdown snapshot history",
		Long: `Manage time-machine snapshots for a markdown file. History is stored under
the OS app data directory (see README).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newSnapshotListCommand(),
		newSnapshotSaveCommand(),
		newSnapshotLabelCommand(),
		newSnapshotReadCommand(),
		newSnapshotDiffCommand(),
	)
	return cmd
}

func newSnapshotListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list <file.md>",
		Short: "List snapshots for a markdown file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			abs, err := AbsMarkdown(args[0])
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			store, err := gateway.NewSnapshotStore(abs)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			meta, err := store.List()
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			if err := enc.Encode(map[string]any{"file": abs, "snapshots": meta}); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}

func newSnapshotSaveCommand() *cobra.Command {
	var label string
	c := &cobra.Command{
		Use:   "save <file.md>",
		Short: "Save the current file contents as a snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			store, err := gateway.NewSnapshotStore(abs)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			id, err := store.WriteSnapshotLabeled(string(raw), label)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			if err := enc.Encode(map[string]any{"ok": true, "id": id, "label": label}); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.Flags().SortFlags = false
	c.Flags().StringVar(&label, "label", "", "optional human-readable label")
	return c
}

func newSnapshotLabelCommand() *cobra.Command {
	var id, label string
	c := &cobra.Command{
		Use:   "label <file.md>",
		Short: "Set or update the label on a snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == "" {
				WriteError(fmt.Errorf("--id is required"))
				return ErrAlreadyReported
			}
			abs, err := AbsMarkdown(args[0])
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			store, err := gateway.NewSnapshotStore(abs)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			if err := store.LabelSnapshot(id, label); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			if err := enc.Encode(map[string]any{"ok": true, "id": id, "label": label}); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.Flags().SortFlags = false
	c.Flags().StringVar(&id, "id", "", "snapshot id (timestamp stem)")
	c.Flags().StringVar(&label, "label", "", "label text")
	return c
}

func newSnapshotReadCommand() *cobra.Command {
	var id string
	c := &cobra.Command{
		Use:   "read <file.md>",
		Short: "Read snapshot contents by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == "" {
				WriteError(fmt.Errorf("--id is required"))
				return ErrAlreadyReported
			}
			abs, err := AbsMarkdown(args[0])
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			store, err := gateway.NewSnapshotStore(abs)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			content, err := store.Read(id)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			if err := enc.Encode(map[string]any{"id": id, "content": content}); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.Flags().SortFlags = false
	c.Flags().StringVar(&id, "id", "", "snapshot id")
	return c
}

func newSnapshotDiffCommand() *cobra.Command {
	var a, b string
	c := &cobra.Command{
		Use:   "diff <file.md>",
		Short: "Diff two snapshots, or a snapshot against the working file",
		Long: `Compare snapshot --a to snapshot --b, or omit --b to diff against the
current file on disk.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if a == "" {
				WriteError(fmt.Errorf("--a is required"))
				return ErrAlreadyReported
			}
			abs, err := AbsMarkdown(args[0])
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			store, err := gateway.NewSnapshotStore(abs)
			if err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			var hunks []gateway.DiffHunk
			if b == "" {
				raw, err := os.ReadFile(abs)
				if err != nil {
					WriteError(err)
					return ErrAlreadyReported
				}
				hunks, err = store.DiffAgainstContent(a, string(raw))
				if err != nil {
					WriteError(err)
					return ErrAlreadyReported
				}
			} else {
				hunks, err = store.Diff(a, b)
				if err != nil {
					WriteError(err)
					return ErrAlreadyReported
				}
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			if err := enc.Encode(gateway.DiffResponse{Hunks: hunks}); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.Flags().SortFlags = false
	c.Flags().StringVar(&a, "a", "", "left snapshot id (older baseline)")
	c.Flags().StringVar(&b, "b", "", "right snapshot id (omit to diff against working file)")
	return c
}
