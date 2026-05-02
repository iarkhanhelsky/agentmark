package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrationSkillCommand(t *testing.T) {
	t.Run("default prints embedded markdown", func(t *testing.T) {
		got := runCLI(t, []string{"skill"})
		if got.err != nil {
			t.Fatalf("skill error: %v stderr=%s", got.err, got.stderr)
		}
		if strings.TrimSpace(got.stdout) == "" {
			t.Fatalf("expected markdown output")
		}
		if !strings.Contains(got.stdout, "# ") {
			t.Fatalf("expected markdown heading in output")
		}
	})

	t.Run("output writes file and json", func(t *testing.T) {
		outPath := filepath.Join(t.TempDir(), "nested", "SKILL.md")
		got := runCLI(t, []string{"skill", "--output", outPath})
		if got.err != nil {
			t.Fatalf("skill output error: %v stderr=%s", got.err, got.stderr)
		}
		res := decodeJSONMap(t, got.stdout)
		if ok, _ := res["ok"].(bool); !ok {
			t.Fatalf("expected ok=true response: %s", got.stdout)
		}
		pathValue, _ := res["path"].(string)
		if pathValue != filepath.Clean(outPath) {
			t.Fatalf("path mismatch got=%q want=%q", pathValue, filepath.Clean(outPath))
		}
		raw, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read output skill: %v", err)
		}
		if strings.TrimSpace(string(raw)) == "" {
			t.Fatalf("expected skill file to be non-empty")
		}
	})

	t.Run("flags are mutually exclusive", func(t *testing.T) {
		outPath := filepath.Join(t.TempDir(), "SKILL.md")
		got := runCLI(t, []string{"skill", "--install-cursor", "--output", outPath})
		if got.err == nil {
			t.Fatalf("expected error for mixed install/output flags")
		}
		if !hasJSONError(got.stderr) {
			t.Fatalf("expected json stderr, got %q", got.stderr)
		}
		if !strings.Contains(got.stderr, "use only one of --install-claude, --install-cursor, --output") {
			t.Fatalf("unexpected stderr %q", got.stderr)
		}
	})
}
