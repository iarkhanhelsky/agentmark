package gateway

// CommentMessage is a single note in a thread (human-authored in this tool).
type CommentMessage struct {
	Role string `json:"role"` // "user" | "agent" (legacy sidecars may contain agent)
	Body string `json:"body"`
	TS   int64  `json:"ts"`
}

// CommentAnchor offsets into the markdown document.
type CommentAnchor struct {
	StartOffset int    `json:"startOffset"`
	EndOffset   int    `json:"endOffset"`
	Prefix      string `json:"prefix"`
	Suffix      string `json:"suffix"`
	LongPrefix  string `json:"longPrefix,omitempty"`
	LongSuffix  string `json:"longSuffix,omitempty"`
}

// CommentThread is one anchored review thread.
type CommentThread struct {
	ID         string         `json:"id"`
	AnchorText string         `json:"anchorText"`
	Anchor     *CommentAnchor `json:"anchor,omitempty"`
	// AnchorNormalized is whitespace-collapsed anchor text for fallback matching.
	AnchorNormalized string `json:"anchorNormalized,omitempty"`
	// AnchorBasisHash is sha256 hex of the full markdown file when this thread last anchored successfully.
	AnchorBasisHash string           `json:"anchorBasisHash,omitempty"`
	Detached        bool             `json:"detached,omitempty"`
	Resolved        bool             `json:"resolved,omitempty"`
	Thread          []CommentMessage `json:"thread"`
}

// CommentsFile is the on-disk sidecar shape.
type CommentsFile struct {
	Version int             `json:"version"`
	Threads []CommentThread `json:"threads"`
}

// SnapshotMeta is returned by GET /api/snapshots.
type SnapshotMeta struct {
	ID    string `json:"id"`
	TS    string `json:"ts"`
	Lines int    `json:"lines"`
	Label string `json:"label,omitempty"`
	Auto  bool   `json:"auto,omitempty"`
}

// DiffHunk is one change block between two snapshots.
type DiffHunk struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`    // "equal" | "delete" | "insert" | "replace"
	OldLine int      `json:"oldLine"` // 1-based start line in old content (0 if N/A)
	NewLine int      `json:"newLine"` // 1-based start line in new content
	Old     []string `json:"oldLines,omitempty"`
	New     []string `json:"newLines,omitempty"`
}

// DiffResponse is GET /api/snapshots/diff.
type DiffResponse struct {
	Hunks []DiffHunk `json:"hunks"`
}
