package server

import (
	"path/filepath"
	"testing"
)

func TestSafeJoin(t *testing.T) {
	root := filepath.Join(t.TempDir(), "proj")

	t.Run("relative file", func(t *testing.T) {
		got, err := SafeJoin(root, "docs/a.md")
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(root, "docs", "a.md")
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("forward slashes", func(t *testing.T) {
		got, err := SafeJoin(root, "docs/a.md")
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(got) != "a.md" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("dotdot rejected", func(t *testing.T) {
		_, err := SafeJoin(root, ".."+string(filepath.Separator)+"etc")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("nested dotdot rejected", func(t *testing.T) {
		_, err := SafeJoin(root, filepath.Join("docs", "..", "..", "outside"))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("absolute inside root", func(t *testing.T) {
		inside := filepath.Join(root, "b.md")
		got, err := SafeJoin(root, inside)
		if err != nil {
			t.Fatal(err)
		}
		if got != inside {
			t.Fatalf("got %q want %q", got, inside)
		}
	})

	t.Run("absolute outside root", func(t *testing.T) {
		outside := filepath.Join(root, "..", "other", "x.md")
		outside = filepath.Clean(outside)
		_, err := SafeJoin(root, outside)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
