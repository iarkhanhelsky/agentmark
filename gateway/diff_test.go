package gateway

import (
	"strings"
	"testing"
)

func TestLineDiff_identical(t *testing.T) {
	h := LineDiff("a\nb\n", "a\nb\n")
	if len(h) != 0 {
		t.Fatalf("expected no hunks, got %#v", h)
	}
}

func TestLineDiff_emptyToContent(t *testing.T) {
	h := LineDiff("", "only new")
	if len(h) != 1 || h[0].Kind != "insert" {
		t.Fatalf("got %#v", h)
	}
	if h[0].OldLine != 1 || h[0].NewLine != 1 {
		t.Fatalf("lines: old=%d new=%d", h[0].OldLine, h[0].NewLine)
	}
	if strings.Join(h[0].New, "\n") != "only new" {
		t.Fatalf("new chunk: %#v", h[0].New)
	}
}

func TestLineDiff_contentToEmpty(t *testing.T) {
	// Single logical line (no trailing newline) so the delete hunk is one line.
	h := LineDiff("gone", "")
	if len(h) != 1 || h[0].Kind != "delete" {
		t.Fatalf("got %#v", h)
	}
	if strings.Join(h[0].Old, "\n") != "gone" {
		t.Fatalf("old chunk: %#v", h[0].Old)
	}
}

func TestLineDiff_replaceAndCRLF(t *testing.T) {
	old := "keep\r\nremove me\r\ntail"
	newText := "keep\r\ninserted\r\ntail"
	h := LineDiff(old, newText)
	if len(h) != 1 || h[0].Kind != "replace" {
		t.Fatalf("got %#v", h)
	}
	if strings.Join(h[0].Old, "\n") != "remove me" || strings.Join(h[0].New, "\n") != "inserted" {
		t.Fatalf("chunks old=%#v new=%#v", h[0].Old, h[0].New)
	}
}

func TestLineDiff_hunkIDs(t *testing.T) {
	h := LineDiff("a\nx\n", "a\ny\nz\n")
	seen := map[string]bool{}
	for _, u := range h {
		if u.ID == "" || seen[u.ID] {
			t.Fatalf("bad id %q", u.ID)
		}
		seen[u.ID] = true
	}
}

func TestApplyHunksToCurrent_replace(t *testing.T) {
	old := "line1\nOLD\nline3"
	newText := "line1\nNEW\nline3"
	hunks := LineDiff(old, newText)
	var accept map[string]struct{}
	for _, u := range hunks {
		if u.Kind == "replace" {
			accept = map[string]struct{}{u.ID: {}}
			break
		}
	}
	if accept == nil {
		t.Fatal("no replace hunk")
	}
	out, err := ApplyHunksToCurrent(old, hunks, accept)
	if err != nil {
		t.Fatal(err)
	}
	if out != newText {
		t.Fatalf("got %q want %q", out, newText)
	}
}

func TestApplyHunksToCurrent_noAcceptedUnchanged(t *testing.T) {
	cur := "a\nb\n"
	h := LineDiff(cur, "a\nB\n")
	out, err := ApplyHunksToCurrent(cur, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != cur {
		t.Fatalf("got %q", out)
	}
}

func TestApplyHunksToCurrent_replaceOldNotFound(t *testing.T) {
	_, err := ApplyHunksToCurrent("no match here", []DiffHunk{{
		ID: "h0", Kind: "replace", OldLine: 1, NewLine: 1,
		Old: []string{"missing"}, New: []string{"x"},
	}}, map[string]struct{}{"h0": {}})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestApplyHunksToCurrent_insertAtStart(t *testing.T) {
	out, err := ApplyHunksToCurrent("body", []DiffHunk{{
		ID: "h0", Kind: "insert", OldLine: 0, NewLine: 1,
		Old: nil, New: []string{"head"},
	}}, map[string]struct{}{"h0": {}})
	if err != nil {
		t.Fatal(err)
	}
	if out != "head\nbody" {
		t.Fatalf("got %q", out)
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

func TestMapOldByteRangeToNew_invalidRange(t *testing.T) {
	for _, tc := range []struct {
		start, end int
	}{
		{-1, 1},
		{0, 0},
		{5, 3},
		{0, 100},
	} {
		_, _, ok := MapOldByteRangeToNew("hello", "hello", tc.start, tc.end)
		if ok {
			t.Fatalf("expected false for start=%d end=%d", tc.start, tc.end)
		}
	}
}

func TestMapOldByteRangeToNew_directSubstring(t *testing.T) {
	old := "alpha beta gamma"
	newText := "prefix alpha beta gamma"
	start := strings.Index(old, "beta")
	end := start + len("beta")
	ns, ne, ok := MapOldByteRangeToNew(old, newText, start, end)
	if !ok {
		t.Fatal("expected ok")
	}
	if newText[ns:ne] != "beta" {
		t.Fatalf("got %q", newText[ns:ne])
	}
}

func TestMapOldByteRangeToNew_deletedLines(t *testing.T) {
	old := "stay\nremove\n"
	newText := "stay\n"
	start := strings.Index(old, "remove")
	end := start + len("remove")
	_, _, ok := MapOldByteRangeToNew(old, newText, start, end)
	if ok {
		t.Fatal("expected false when mapped lines deleted")
	}
}
