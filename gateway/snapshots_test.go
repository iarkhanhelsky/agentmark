package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewSnapshotStore_dirUnderDataRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENTMARK_DATA_DIR", root)
	mdPath := filepath.Join(t.TempDir(), "notes.md")
	store, err := NewSnapshotStore(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(store.Dir(), filepath.Join(root, "agentmark", "history")) {
		t.Fatalf("Dir=%q", store.Dir())
	}
}

func TestSnapshotStore_ListReadWrite(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	mdPath := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(mdPath, []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := NewSnapshotStore(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}

	id1, err := store.WriteSnapshotNow("first")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	id2, err := store.WriteSnapshotNow("second")
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Fatal("expected distinct snapshot ids")
	}

	list, err = store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 entries, got %d", len(list))
	}
	if list[0].ID != id2 {
		t.Fatalf("newest-first: first id %q want %q", list[0].ID, id2)
	}
	if list[0].Lines != 1 || !list[0].Auto {
		t.Fatalf("meta: %+v", list[0])
	}

	body, err := store.Read(id2)
	if err != nil || body != "second" {
		t.Fatalf("Read: err=%v body=%q", err, body)
	}
}

func TestSnapshotStore_Read_invalidID(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	mdPath := filepath.Join(t.TempDir(), "doc.md")
	store, err := NewSnapshotStore(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../escape", `a\b`} {
		if _, err := store.Read(id); err == nil {
			t.Fatalf("expected error for id %q", id)
		}
	}
}

func TestSnapshotStore_ReadMatchingContentHash(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	mdPath := filepath.Join(t.TempDir(), "doc.md")
	store, err := NewSnapshotStore(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	content := "payload\n"
	if _, err := store.WriteSnapshotNow(content); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	hexWant := hex.EncodeToString(sum[:])
	got, ok := store.ReadMatchingContentHash(hexWant)
	if !ok || got != content {
		t.Fatalf("ok=%v got=%q", ok, got)
	}
	if _, ok := store.ReadMatchingContentHash(""); ok {
		t.Fatal("empty hash should miss")
	}
	if _, ok := store.ReadMatchingContentHash("deadbeef"); ok {
		t.Fatal("wrong hash should miss")
	}
}

func TestSnapshotStore_WriteSnapshotLabeled_andLabelSnapshot(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	mdPath := filepath.Join(t.TempDir(), "doc.md")
	store, err := NewSnapshotStore(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.WriteSnapshotLabeled("v\n", "milestone")
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Label != "milestone" || list[0].Auto {
		t.Fatalf("list: %+v", list[0])
	}
	if err := store.LabelSnapshot(id, "renamed"); err != nil {
		t.Fatal(err)
	}
	list, err = store.List()
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Label != "renamed" {
		t.Fatalf("label=%q", list[0].Label)
	}
}

func TestSnapshotStore_LabelSnapshot_invalidID(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	mdPath := filepath.Join(t.TempDir(), "doc.md")
	store, err := NewSnapshotStore(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.LabelSnapshot("../x", "nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSnapshotStore_Diff(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	mdPath := filepath.Join(t.TempDir(), "doc.md")
	store, err := NewSnapshotStore(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	left, err := store.WriteSnapshotNow("a\nOLD\nb\n")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	right, err := store.WriteSnapshotNow("a\nNEW\nb\n")
	if err != nil {
		t.Fatal(err)
	}
	hunks, err := store.Diff(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if len(hunks) != 1 || hunks[0].Kind != "replace" {
		t.Fatalf("hunks=%#v", hunks)
	}
}

func TestSnapshotStore_DiffAgainstContent(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	mdPath := filepath.Join(t.TempDir(), "doc.md")
	store, err := NewSnapshotStore(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.WriteSnapshotNow("one\ntwo\n")
	if err != nil {
		t.Fatal(err)
	}
	hunks, err := store.DiffAgainstContent(id, "one\nTWO\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(hunks) != 1 {
		t.Fatalf("hunks=%#v", hunks)
	}
}

func TestSnapshotStore_pruneKeepsMaxSnapshots(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	mdPath := filepath.Join(t.TempDir(), "doc.md")
	store, err := NewSnapshotStore(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxSnapshots+5; i++ {
		if _, err := store.WriteSnapshotNow(strings.Repeat("x", i+1) + "\n"); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	names, err := store.listNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != maxSnapshots {
		t.Fatalf("after prune want %d files, got %d", maxSnapshots, len(names))
	}
}
