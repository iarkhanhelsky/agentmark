package server

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
