package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeAnchorText(t *testing.T) {
	if g, e := NormalizeAnchorText("  a \n\t b  "), "a b"; g != e {
		t.Fatalf("got %q want %q", g, e)
	}
}

func TestReanchorThreads_normalizedWhitespace(t *testing.T) {
	md := "Hello   world\n\nfoo"
	threads := []CommentThread{{
		ID:         "t1",
		AnchorText: "Hello world",
		Anchor:     &CommentAnchor{StartOffset: 0, EndOffset: 11},
	}}
	out := ReanchorThreads(md, threads)
	if len(out) != 1 || out[0].Detached {
		t.Fatalf("expected attached: %+v", out[0])
	}
	if !strings.Contains(out[0].AnchorText, "Hello") || !strings.Contains(out[0].AnchorText, "world") {
		t.Fatalf("anchor text: %q", out[0].AnchorText)
	}
}

func TestReanchorThreads_bracketMiddleChanged(t *testing.T) {
	old := "PREFIX keep start MIDDLE end SUFFIX tail"
	md := "PREFIX keep start NEWPHRASE end SUFFIX tail"
	an := BuildAnchor(old, strings.Index(old, "MIDDLE"), strings.Index(old, "MIDDLE")+len("MIDDLE"))
	threads := []CommentThread{{
		ID:         "t1",
		AnchorText: "MIDDLE",
		Anchor:     &an,
	}}
	out := ReanchorThreads(md, threads)
	if len(out) != 1 || out[0].Detached {
		t.Fatalf("expected bracket match: %+v", out[0])
	}
	if out[0].AnchorText != "NEWPHRASE" {
		t.Fatalf("got anchor %q", out[0].AnchorText)
	}
}

func TestMapOldByteRangeToNew_lineShift(t *testing.T) {
	old := "line0\nkeep this phrase\nline2"
	newText := "inserted\nline0\nkeep this phrase\nline2"
	start := strings.Index(old, "keep this phrase")
	end := start + len("keep this phrase")
	ns, ne, ok := MapOldByteRangeToNew(old, newText, start, end)
	if !ok {
		t.Fatal("expected ok")
	}
	if newText[ns:ne] != "keep this phrase" {
		t.Fatalf("got %q", newText[ns:ne])
	}
}

func TestReanchorThreadsWithStore_snapshotBasisHash(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	tmpdir := t.TempDir()
	mdPath := filepath.Join(tmpdir, "doc.md")
	v1 := "# Title\n\nkeep this anchor phrase\n"
	v2 := "# Title\n\nedited before keep this anchor phrase after\n"
	if err := os.WriteFile(mdPath, []byte(v1), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := NewSnapshotStore(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.WriteSnapshotNow(v1); err != nil {
		t.Fatal(err)
	}
	start := strings.Index(v1, "keep this anchor phrase")
	end := start + len("keep this anchor phrase")
	an := BuildAnchor(v1, start, end)
	hash := contentHashHex(v1)
	threads := []CommentThread{{
		ID:              "t1",
		AnchorText:      "typo phrase removed from file",
		Anchor:          &an,
		AnchorBasisHash: hash,
		Detached:        true,
	}}
	if err := os.WriteFile(mdPath, []byte(v2), 0o644); err != nil {
		t.Fatal(err)
	}
	out := ReanchorThreadsWithStore(v2, threads, store)
	if len(out) != 1 || out[0].Detached {
		t.Fatalf("expected snapshot remap: %+v", out[0])
	}
	if !strings.Contains(out[0].AnchorText, "keep this anchor phrase") {
		t.Fatalf("anchor: %q", out[0].AnchorText)
	}
}

func TestReanchorThreadsWithStore_dedupesSameIDPrefersAttached(t *testing.T) {
	md := "# Doc\n\nAnchor text\n"
	threads := []CommentThread{
		{
			ID:         "dup-1",
			AnchorText: "missing text",
			Detached:   true,
			Thread: []CommentMessage{
				{Role: "agent", Body: "first", TS: 1},
			},
		},
		{
			ID:         "dup-1",
			AnchorText: "Anchor text",
			Thread: []CommentMessage{
				{Role: "agent", Body: "second", TS: 2},
			},
		},
	}

	out := ReanchorThreadsWithStore(md, threads, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 deduped thread, got %d", len(out))
	}
	if out[0].ID != "dup-1" {
		t.Fatalf("unexpected id %q", out[0].ID)
	}
	if out[0].Detached {
		t.Fatalf("expected attached canonical thread, got detached: %+v", out[0])
	}
	if out[0].AnchorText != "Anchor text" {
		t.Fatalf("expected canonical anchor text, got %q", out[0].AnchorText)
	}
}
