package cli

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed skill.md
var skillMarkdown string

func newSkillCommand() *cobra.Command {
	var installClaude, installCursor bool
	var outPath string

	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Print or install the AgentMark agent skill (SKILL.md)",
		Long: `Print the embedded SKILL.md to stdout, or install it to a standard
location for Claude Code or Cursor agent skills.`,
		Example: `  agentmark skill
  agentmark skill --install-cursor
  agentmark skill --output ~/tmp/SKILL.md`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			nInstall := 0
			if installClaude {
				nInstall++
			}
			if installCursor {
				nInstall++
			}
			if outPath != "" {
				nInstall++
			}
			if nInstall > 1 {
				WriteError(fmt.Errorf("use only one of --install-claude, --install-cursor, --output"))
				return ErrAlreadyReported
			}

			var dest string
			switch {
			case installClaude:
				home, err := os.UserHomeDir()
				if err != nil {
					WriteError(err)
					return ErrAlreadyReported
				}
				dest = filepath.Join(home, ".claude", "skills", "agentmark", "SKILL.md")
			case installCursor:
				home, err := os.UserHomeDir()
				if err != nil {
					WriteError(err)
					return ErrAlreadyReported
				}
				dest = filepath.Join(home, ".cursor", "skills-cursor", "agentmark", "SKILL.md")
			case outPath != "":
				var err error
				dest, err = ExpandUser(outPath)
				if err != nil {
					WriteError(err)
					return ErrAlreadyReported
				}
				dest = filepath.Clean(dest)
			default:
				_, _ = os.Stdout.WriteString(skillMarkdown)
				if len(skillMarkdown) > 0 && skillMarkdown[len(skillMarkdown)-1] != '\n' {
					_, _ = os.Stdout.WriteString("\n")
				}
				return nil
			}

			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			if err := os.WriteFile(dest, []byte(skillMarkdown), 0o644); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			if err := enc.Encode(map[string]any{"ok": true, "path": dest}); err != nil {
				WriteError(err)
				return ErrAlreadyReported
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Flags().SortFlags = false
	cmd.Flags().BoolVar(&installClaude, "install-claude", false, "write to ~/.claude/skills/agentmark/SKILL.md")
	cmd.Flags().BoolVar(&installCursor, "install-cursor", false, "write to ~/.cursor/skills-cursor/agentmark/SKILL.md")
	cmd.Flags().StringVar(&outPath, "output", "", "write SKILL.md to this path")
	return cmd
}
