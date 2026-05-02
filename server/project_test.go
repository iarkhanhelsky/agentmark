package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildProjectTreeGrouping(t *testing.T) {
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(root, "root.md"), []byte("# r\n"), 0o644))
	must(os.MkdirAll(filepath.Join(root, "docs"), 0o755))
	must(os.WriteFile(filepath.Join(root, "docs", "a.md"), []byte("# a\n"), 0o644))
	must(os.MkdirAll(filepath.Join(root, "Drafts"), 0o755))
	must(os.WriteFile(filepath.Join(root, "Drafts", "b.md"), []byte("# b\n"), 0o644))

	tree, err := buildProjectTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Root != filepath.Clean(root) {
		t.Fatalf("root %q", tree.Root)
	}
	if len(tree.Sections) != 3 {
		t.Fatalf("sections %d", len(tree.Sections))
	}
	// Empty label (root files) sorts first
	if tree.Sections[0].Label != "" || len(tree.Sections[0].Files) != 1 {
		t.Fatalf("root section %+v", tree.Sections[0])
	}
	seen := map[string]int{}
	for _, s := range tree.Sections[1:] {
		seen[s.Label] = len(s.Files)
	}
	if seen["docs"] != 1 || seen["Drafts"] != 1 {
		t.Fatalf("expected docs and Drafts sections, got %v", seen)
	}
}

func TestPickInitialMarkdown(t *testing.T) {
	t.Run("prefers_readme", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "z.md"), []byte("z"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "Readme.md"), []byte("r"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := PickInitialMarkdown(root)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(got) != "Readme.md" {
			t.Fatalf("got %s", got)
		}
	})

	t.Run("recursive_when_no_root_md", func(t *testing.T) {
		root := t.TempDir()
		sub := filepath.Join(root, "inner")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "only.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := PickInitialMarkdown(root)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(filepath.ToSlash(got), "inner/only.md") {
			t.Fatalf("got %s", got)
		}
	})

	t.Run("creates_readme_when_empty", func(t *testing.T) {
		root := t.TempDir()
		got, err := PickInitialMarkdown(root)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(got) != "README.md" {
			t.Fatalf("got %s", got)
		}
		if _, err := os.Stat(got); err != nil {
			t.Fatal(err)
		}
	})
}
