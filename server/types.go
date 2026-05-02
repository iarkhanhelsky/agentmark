package server

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

// UpsertThreadRequest is POST /api/threads/upsert body.
type UpsertThreadRequest struct {
	ID         string         `json:"id"`
	AnchorText string         `json:"anchorText"`
	Anchor     *CommentAnchor `json:"anchor,omitempty"`
	Message    string         `json:"message,omitempty"`
}

// WSEvent is a JSON message pushed over WebSocket.
type WSEvent struct {
	Type string `json:"type"`
	// file_update
	Content string `json:"content,omitempty"`
	// threads_update
	Threads []CommentThread `json:"threads,omitempty"`
	// snapshot_saved
	SnapshotID string `json:"id,omitempty"`
	Timestamp  string `json:"ts,omitempty"`
	// active_file
	Name    string `json:"name,omitempty"`
	Path    string `json:"path,omitempty"`
	RelPath string `json:"relPath,omitempty"`
	// app_reload
	Reason string `json:"reason,omitempty"`
}

// ProjectTreeFile is one markdown file in GET /api/project/tree.
type ProjectTreeFile struct {
	RelPath          string `json:"relPath"`
	Name             string `json:"name"`
	OpenCommentCount int    `json:"openCommentCount"`
}

// ProjectTreeSection groups files by top-level directory under the project root.
type ProjectTreeSection struct {
	Label string            `json:"label"`
	Files []ProjectTreeFile `json:"files"`
}

// ProjectTreeResponse is GET /api/project/tree.
type ProjectTreeResponse struct {
	Root     string               `json:"root"`
	Sections []ProjectTreeSection `json:"sections"`
}

// SelectFileRequest is POST /api/file/select body.
type SelectFileRequest struct {
	Path string `json:"path"`
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

// ApplyRequest is POST /api/snapshots/apply.
type ApplyRequest struct {
	LeftID          string   `json:"leftId"`
	RightID         string   `json:"rightId"`
	AcceptedHunkIDs []string `json:"acceptedHunkIds"`
}

// ThreadReplyRequest is POST /api/threads/reply.
type ThreadReplyRequest struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}
