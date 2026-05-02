package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// CommentsPathFor returns the sidecar path for a markdown file.
func CommentsPathFor(filePath string) string {
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)
	return filepath.Join(dir, "."+base+".comments.json")
}

// LoadThreads reads comment threads from the sidecar; missing file => empty.
func LoadThreads(filePath string) ([]CommentThread, error) {
	p := CommentsPathFor(filePath)
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f CommentsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	if f.Threads == nil {
		return nil, nil
	}
	return f.Threads, nil
}

// SaveThreads writes threads to the sidecar.
func SaveThreads(filePath string, threads []CommentThread) error {
	p := CommentsPathFor(filePath)
	f := CommentsFile{Version: 1, Threads: threads}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(raw, '\n'), 0o644)
}

func occurrenceIndices(haystack, needle string) []int {
	if needle == "" {
		return nil
	}
	var out []int
	from := 0
	for from < len(haystack) {
		idx := strings.Index(haystack[from:], needle)
		if idx < 0 {
			break
		}
		abs := from + idx
		out = append(out, abs)
		step := len(needle)
		if step < 1 {
			step = 1
		}
		from = abs + step
	}
	return out
}

func buildAnchor(markdown string, startOffset, endOffset int) CommentAnchor {
	prefixStart := max(0, startOffset-40)
	suffixEnd := min(len(markdown), endOffset+40)
	return CommentAnchor{
		StartOffset: startOffset,
		EndOffset:   endOffset,
		Prefix:      markdown[prefixStart:startOffset],
		Suffix:      markdown[endOffset:suffixEnd],
	}
}

func chooseBestAnchorIndex(markdown string, thread CommentThread) int {
	anchorText := strings.TrimSpace(thread.AnchorText)
	candidates := occurrenceIndices(markdown, anchorText)
	if len(candidates) == 0 {
		return -1
	}
	if len(candidates) == 1 || thread.Anchor == nil {
		return candidates[0]
	}
	type scored struct {
		start int
		score float64
	}
	var best scored
	first := true
	a := thread.Anchor
	for _, startOffset := range candidates {
		endOffset := startOffset + len(anchorText)
		prefixStart := max(0, startOffset-40)
		suffixEnd := min(len(markdown), endOffset+40)
		prefix := markdown[prefixStart:startOffset]
		suffix := markdown[endOffset:suffixEnd]
		prefixHit := 0.0
		if strings.HasSuffix(prefix, a.Prefix) {
			prefixHit = 1
		}
		suffixHit := 0.0
		if strings.HasPrefix(suffix, a.Suffix) {
			suffixHit = 1
		}
		distance := abs(startOffset - a.StartOffset)
		score := prefixHit*2 + suffixHit*2 - float64(distance)/1000
		if first || score > best.score {
			best = scored{startOffset, score}
			first = false
		}
	}
	return best.start
}

func resolveAnchorLocation(markdown string, thread CommentThread) (startOffset, endOffset int, ok bool) {
	if strings.TrimSpace(thread.AnchorText) == "" || markdown == "" {
		return 0, 0, false
	}
	start := chooseBestAnchorIndex(markdown, thread)
	if start < 0 {
		return 0, 0, false
	}
	trimmed := strings.TrimSpace(thread.AnchorText)
	end := start + len(trimmed)
	if end > len(markdown) {
		end = len(markdown)
	}
	return start, end, true
}

// ReanchorThreads updates anchors and detached flags from current markdown.
func ReanchorThreads(markdown string, input []CommentThread) []CommentThread {
	out := make([]CommentThread, len(input))
	for i, thread := range input {
		start, end, ok := resolveAnchorLocation(markdown, thread)
		if !ok {
			t := thread
			t.Detached = true
			out[i] = t
			continue
		}
		t := thread
		t.Detached = false
		a := buildAnchor(markdown, start, end)
		t.Anchor = &a
		out[i] = t
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
