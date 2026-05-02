package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const anchorWindowShort = 40
const anchorWindowLong = 120

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

// OpenUnresolvedThreadCount returns the number of threads that are not resolved
// and not detached for the given markdown file (reads sidecar).
func OpenUnresolvedThreadCount(filePath string) (int, error) {
	threads, err := LoadThreads(filePath)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, t := range threads {
		if !t.Resolved && !t.Detached {
			n++
		}
	}
	return n, nil
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

// NormalizeAnchorText collapses whitespace to single spaces (for fallback matching).
func NormalizeAnchorText(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func contentHashHex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// collapseWhitespaceWithIndex builds a whitespace-collapsed view of s and a parallel
// byteIndex where byteIndex[i] is the byte offset in s for collapsed[i].
func collapseWhitespaceWithIndex(s string) (collapsed string, byteIndex []int) {
	var b strings.Builder
	byteIndex = make([]int, 0, len(s))
	inWS := false
	for i := 0; i < len(s); {
		r, w := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && w == 1 {
			i++
			continue
		}
		if unicode.IsSpace(r) {
			if !inWS {
				b.WriteByte(' ')
				byteIndex = append(byteIndex, i)
				inWS = true
			}
			i += w
			continue
		}
		inWS = false
		for j := 0; j < w; j++ {
			byteIndex = append(byteIndex, i+j)
		}
		b.WriteString(s[i : i+w])
		i += w
	}
	return b.String(), byteIndex
}

// BuildAnchor captures prefix/suffix context around a span in markdown.
func BuildAnchor(markdown string, startOffset, endOffset int) CommentAnchor {
	prefixStart := max(0, startOffset-anchorWindowShort)
	suffixEnd := min(len(markdown), endOffset+anchorWindowShort)
	longPre := max(0, startOffset-anchorWindowLong)
	longSuf := min(len(markdown), endOffset+anchorWindowLong)
	return CommentAnchor{
		StartOffset: startOffset,
		EndOffset:   endOffset,
		Prefix:      markdown[prefixStart:startOffset],
		Suffix:      markdown[endOffset:suffixEnd],
		LongPrefix:  markdown[longPre:startOffset],
		LongSuffix:  markdown[endOffset:longSuf],
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
		prefixStart := max(0, startOffset-anchorWindowShort)
		suffixEnd := min(len(markdown), endOffset+anchorWindowShort)
		prefix := markdown[prefixStart:startOffset]
		suffix := markdown[endOffset:suffixEnd]
		prefixHit := 0.0
		if strings.HasSuffix(prefix, a.Prefix) {
			prefixHit = 1
		}
		if prefixHit < 1 && len(a.LongPrefix) >= 8 && len(prefix) >= 8 {
			tail := a.LongPrefix
			if len(tail) > 24 {
				tail = tail[len(tail)-24:]
			}
			if strings.HasSuffix(prefix, tail) {
				prefixHit = 1
			}
		}
		suffixHit := 0.0
		if strings.HasPrefix(suffix, a.Suffix) {
			suffixHit = 1
		}
		if suffixHit < 1 && len(a.LongSuffix) >= 8 && len(suffix) >= 8 {
			head := a.LongSuffix
			if len(head) > 24 {
				head = head[:24]
			}
			if strings.HasPrefix(suffix, head) {
				suffixHit = 1
			}
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

func resolveNormalizedAnchored(markdown, needleNorm string) (startOffset, endOffset int, ok bool) {
	needleNorm = strings.TrimSpace(needleNorm)
	if needleNorm == "" || markdown == "" {
		return 0, 0, false
	}
	collapsedDoc, idxDoc := collapseWhitespaceWithIndex(markdown)
	if len(collapsedDoc) != len(idxDoc) {
		return 0, 0, false
	}
	sub := strings.Index(collapsedDoc, needleNorm)
	if sub < 0 {
		return 0, 0, false
	}
	endSub := sub + len(needleNorm)
	if endSub > len(collapsedDoc) || sub < 0 {
		return 0, 0, false
	}
	startByte := idxDoc[sub]
	endByte := idxDoc[endSub-1] + 1
	if startByte < 0 || endByte > len(markdown) || startByte >= endByte {
		return 0, 0, false
	}
	return startByte, endByte, true
}

func resolveBracketAnchored(markdown string, thread CommentThread) (startOffset, endOffset int, ok bool) {
	a := thread.Anchor
	if a == nil {
		return 0, 0, false
	}
	pre := a.LongPrefix
	suf := a.LongSuffix
	if len(pre) < 8 {
		pre = a.Prefix
	}
	if len(suf) < 8 {
		suf = a.Suffix
	}
	if len(pre) < 8 || len(suf) < 8 {
		return 0, 0, false
	}
	const maxSpan = 8000
	var hits [][2]int
	searchFrom := 0
	for {
		i := strings.Index(markdown[searchFrom:], pre)
		if i < 0 {
			break
		}
		abs := searchFrom + i
		afterPre := abs + len(pre)
		rest := markdown[afterPre:]
		j := strings.Index(rest, suf)
		if j >= 0 && j <= maxSpan {
			innerStart := afterPre
			innerEnd := afterPre + j
			if innerEnd > innerStart {
				hits = append(hits, [2]int{innerStart, innerEnd})
			}
		}
		searchFrom = abs + 1
	}
	if len(hits) != 1 {
		return 0, 0, false
	}
	return hits[0][0], hits[0][1], true
}

func tryRemapFromSnapshot(snap *SnapshotStore, thread CommentThread, newMarkdown string) (startOffset, endOffset int, ok bool) {
	if snap == nil || thread.AnchorBasisHash == "" || thread.Anchor == nil {
		return 0, 0, false
	}
	oldContent, found := snap.ReadMatchingContentHash(thread.AnchorBasisHash)
	if !found {
		return 0, 0, false
	}
	a := thread.Anchor
	if a.StartOffset < 0 || a.EndOffset > len(oldContent) || a.StartOffset >= a.EndOffset {
		return 0, 0, false
	}
	return MapOldByteRangeToNew(oldContent, newMarkdown, a.StartOffset, a.EndOffset)
}

func reanchorOneThread(markdown string, thread CommentThread, snap *SnapshotStore) CommentThread {
	hNorm := strings.TrimSpace(thread.AnchorNormalized)
	if hNorm == "" {
		hNorm = NormalizeAnchorText(thread.AnchorText)
	}

	start, end, ok := resolveAnchorLocation(markdown, thread)
	if ok {
		return finalizeReanchored(thread, markdown, start, end, hNorm)
	}
	if hNorm != "" {
		if start, end, ok = resolveNormalizedAnchored(markdown, hNorm); ok {
			return finalizeReanchored(thread, markdown, start, end, hNorm)
		}
	}
	if start, end, ok = resolveBracketAnchored(markdown, thread); ok {
		return finalizeReanchored(thread, markdown, start, end, hNorm)
	}
	if start, end, ok = tryRemapFromSnapshot(snap, thread, markdown); ok {
		return finalizeReanchored(thread, markdown, start, end, hNorm)
	}

	t := thread
	t.Detached = true
	return t
}

func finalizeReanchored(thread CommentThread, markdown string, start, end int, hNorm string) CommentThread {
	t := thread
	t.Detached = false
	if end > len(markdown) {
		end = len(markdown)
	}
	if start < 0 || start >= end {
		t.Detached = true
		return t
	}
	t.AnchorText = markdown[start:end]
	an := BuildAnchor(markdown, start, end)
	t.Anchor = &an
	if hNorm == "" {
		t.AnchorNormalized = NormalizeAnchorText(t.AnchorText)
	} else {
		t.AnchorNormalized = hNorm
	}
	t.AnchorBasisHash = contentHashHex(markdown)
	return t
}

// ReanchorThreads updates anchors and detached flags from current markdown.
func ReanchorThreads(markdown string, input []CommentThread) []CommentThread {
	return ReanchorThreadsWithStore(markdown, input, nil)
}

// ReanchorThreadsWithStore is like ReanchorThreads but uses snapshot history to remap offsets
// when AnchorBasisHash matches a stored version.
func ReanchorThreadsWithStore(markdown string, input []CommentThread, snap *SnapshotStore) []CommentThread {
	out := make([]CommentThread, len(input))
	for i := range input {
		out[i] = reanchorOneThread(markdown, input[i], snap)
	}
	return dedupeThreadsByID(out)
}

func dedupeThreadsByID(input []CommentThread) []CommentThread {
	if len(input) <= 1 {
		return input
	}
	order := make([]string, 0, len(input))
	byID := make(map[string]CommentThread, len(input))
	for _, t := range input {
		if strings.TrimSpace(t.ID) == "" {
			continue
		}
		cur, exists := byID[t.ID]
		if !exists {
			order = append(order, t.ID)
			byID[t.ID] = t
			continue
		}
		byID[t.ID] = pickCanonicalThread(cur, t)
	}
	out := make([]CommentThread, 0, len(order))
	for _, id := range order {
		if t, ok := byID[id]; ok {
			out = append(out, t)
		}
	}
	return out
}

func pickCanonicalThread(a, b CommentThread) CommentThread {
	score := func(t CommentThread) int {
		s := len(t.Thread) * 100
		if !t.Detached {
			s += 10
		}
		if t.Anchor != nil {
			s += 2
		}
		if strings.TrimSpace(t.AnchorText) != "" {
			s++
		}
		return s
	}
	if score(b) > score(a) {
		return b
	}
	return a
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
