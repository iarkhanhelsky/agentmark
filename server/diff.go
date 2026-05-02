package server

import (
	"fmt"
	"strings"
)

// LineDiff computes a simple line-based diff using LCS (O(n*m), fine for docs).
func LineDiff(oldText, newText string) []DiffHunk {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	lcs := lcsTable(oldLines, newLines)
	ops := walkLCS(oldLines, newLines, lcs)
	return opsToHunks(ops, oldLines, newLines)
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

func lcsTable(a, b []string) [][]int {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				dp[i][j] = max(dp[i+1][j], dp[i][j+1])
			}
		}
	}
	return dp
}

type diffOpKind int

const (
	opEqual diffOpKind = iota
	opDelete
	opInsert
)

type diffOp struct {
	kind diffOpKind
	i, j int // indices in old/new
}

func walkLCS(oldLines, newLines []string, dp [][]int) []diffOp {
	var ops []diffOp
	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		if oldLines[i] == newLines[j] {
			ops = append(ops, diffOp{kind: opEqual, i: i, j: j})
			i++
			j++
			continue
		}
		if dp[i+1][j] >= dp[i][j+1] {
			ops = append(ops, diffOp{kind: opDelete, i: i, j: j})
			i++
		} else {
			ops = append(ops, diffOp{kind: opInsert, i: i, j: j})
			j++
		}
	}
	for i < len(oldLines) {
		ops = append(ops, diffOp{kind: opDelete, i: i, j: j})
		i++
	}
	for j < len(newLines) {
		ops = append(ops, diffOp{kind: opInsert, i: i, j: j})
		j++
	}
	return ops
}

func opsToHunks(ops []diffOp, oldLines, newLines []string) []DiffHunk {
	var hunks []DiffHunk
	n := 0
	for idx := 0; idx < len(ops); {
		op := ops[idx]
		if op.kind == opEqual {
			idx++
			continue
		}
		var oldChunk []string
		var newChunk []string
		oldStart := op.i + 1
		newStart := op.j + 1
		for idx < len(ops) && ops[idx].kind != opEqual {
			switch ops[idx].kind {
			case opDelete:
				oldChunk = append(oldChunk, oldLines[ops[idx].i])
			case opInsert:
				newChunk = append(newChunk, newLines[ops[idx].j])
			}
			idx++
		}
		kind := "replace"
		if len(oldChunk) == 0 {
			kind = "insert"
		}
		if len(newChunk) == 0 {
			kind = "delete"
		}
		hunks = append(hunks, DiffHunk{
			ID:      fmt.Sprintf("h%d", n),
			Kind:    kind,
			OldLine: oldStart,
			NewLine: newStart,
			Old:     oldChunk,
			New:     newChunk,
		})
		n++
	}
	return hunks
}

// ApplyHunksToCurrent applies selected change hunks from diff(left, right) onto current.
// Each hunk replaces the first occurrence of strings.Join(hunk.Old, "\n") with strings.Join(hunk.New, "\n").
// Order: apply from bottom of file to top (by old line) to reduce offset shifts — approximate by sorting hunks by OldLine desc.
func ApplyHunksToCurrent(current string, hunks []DiffHunk, acceptIDs map[string]struct{}) (string, error) {
	// Filter and sort by OldLine descending (replace/delete); inserts use OldLine 0 — apply those last
	var toApply []DiffHunk
	for _, h := range hunks {
		if _, ok := acceptIDs[h.ID]; !ok {
			continue
		}
		if h.Kind == "equal" {
			continue
		}
		toApply = append(toApply, h)
	}
	// Stable sort: deletes/replaces first (high line), inserts last
	for i := 0; i < len(toApply); i++ {
		for j := i + 1; j < len(toApply); j++ {
			if lineKey(toApply[i]) < lineKey(toApply[j]) {
				toApply[i], toApply[j] = toApply[j], toApply[i]
			}
		}
	}

	out := current
	for _, h := range toApply {
		oldJoin := strings.Join(h.Old, "\n")
		newJoin := strings.Join(h.New, "\n")
		switch h.Kind {
		case "delete":
			if oldJoin == "" {
				continue
			}
			idx := strings.Index(out, oldJoin)
			if idx < 0 {
				return "", fmt.Errorf("hunk %s: old text not found in current file", h.ID)
			}
			out = out[:idx] + out[idx+len(oldJoin):]
		case "insert":
			// Insert after the line corresponding to OldLine-1 in current — fragile; use anchor from newLine
			// Fallback: append if OldLine <= 1
			if h.NewLine <= 1 {
				if out == "" {
					out = newJoin
				} else {
					out = newJoin + "\n" + out
				}
				continue
			}
			lines := splitLines(out)
			at := min(h.NewLine-1, len(lines))
			var b strings.Builder
			for i, ln := range lines {
				if i == at {
					b.WriteString(newJoin)
					b.WriteByte('\n')
				}
				b.WriteString(ln)
				if i < len(lines)-1 {
					b.WriteByte('\n')
				}
			}
			if at >= len(lines) {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(newJoin)
			}
			out = strings.TrimSuffix(b.String(), "\n")
		case "replace":
			if oldJoin == "" {
				return "", fmt.Errorf("hunk %s: empty old in replace", h.ID)
			}
			idx := strings.Index(out, oldJoin)
			if idx < 0 {
				return "", fmt.Errorf("hunk %s: old text not found in current file", h.ID)
			}
			out = out[:idx] + newJoin + out[idx+len(oldJoin):]
		}
	}
	return out, nil
}

func lineKey(h DiffHunk) int {
	if h.Kind == "insert" {
		return -1
	}
	return h.OldLine
}

// oldToNewLineIndex maps each old line index to its corresponding new line index (-1 if deleted).
func oldToNewLineIndex(oldLines, newLines []string) []int {
	out := make([]int, len(oldLines))
	for i := range out {
		out[i] = -1
	}
	lcs := lcsTable(oldLines, newLines)
	ops := walkLCS(oldLines, newLines, lcs)
	for _, op := range ops {
		if op.kind == opEqual && op.i >= 0 && op.i < len(out) {
			out[op.i] = op.j
		}
	}
	return out
}

func byteOffsetToLineIndex(text string, byteOff int) int {
	if byteOff < 0 || byteOff > len(text) {
		return -1
	}
	line := 0
	for i := 0; i < byteOff && i < len(text); i++ {
		if text[i] == '\n' {
			line++
		}
	}
	return line
}

func lineStartByteOffset(text string, lineIdx int) int {
	if lineIdx <= 0 {
		return 0
	}
	line := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			line++
			if line == lineIdx {
				return i + 1
			}
		}
	}
	return len(text)
}

// MapOldByteRangeToNew maps half-open byte range [start,end) in oldText into newText using line
// alignment, after trying a direct substring carry-over.
func MapOldByteRangeToNew(oldText, newText string, start, end int) (int, int, bool) {
	if start < 0 || end > len(oldText) || start >= end {
		return 0, 0, false
	}
	snippet := oldText[start:end]
	if i := strings.Index(newText, snippet); i >= 0 {
		return i, i + len(snippet), true
	}
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	lineMap := oldToNewLineIndex(oldLines, newLines)
	loLine := byteOffsetToLineIndex(oldText, start)
	hiLine := byteOffsetToLineIndex(oldText, end-1)
	if loLine < 0 || hiLine < 0 || loLine >= len(lineMap) || hiLine >= len(lineMap) {
		return 0, 0, false
	}
	nLo := lineMap[loLine]
	nHi := lineMap[hiLine]
	if nLo < 0 || nHi < 0 {
		return 0, 0, false
	}
	if nHi < nLo {
		nLo, nHi = nHi, nLo
	}
	regionStart := lineStartByteOffset(newText, nLo)
	regionEnd := lineStartByteOffset(newText, nHi+1)
	if regionEnd > len(newText) {
		regionEnd = len(newText)
	}
	if regionStart > regionEnd || regionStart >= len(newText) {
		return 0, 0, false
	}
	region := newText[regionStart:regionEnd]
	trimSnip := strings.TrimSpace(snippet)
	if trimSnip == "" {
		return 0, 0, false
	}
	idx := strings.Index(region, trimSnip)
	if idx < 0 {
		return 0, 0, false
	}
	ns := regionStart + idx
	ne := ns + len(trimSnip)
	if ne > len(newText) {
		return 0, 0, false
	}
	return ns, ne, true
}
