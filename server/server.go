package server

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

//go:embed all:static
var staticFS embed.FS

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Config for the HTTP server.
type Config struct {
	FilePath    string // absolute path to markdown file
	ProjectRoot string // absolute project directory (parent of initial file); set in Start
	Port        string // e.g. "4173"
}

// App holds runtime state.
type App struct {
	cfg       Config
	content   string
	threads   []CommentThread
	snapshots *SnapshotStore
	clients   map[*websocket.Conn]struct{}
	mu        sync.Mutex
	wsMu      sync.Mutex
	stopWatch func()
	broadcast func(WSEvent)
}

// Start runs the HTTP server; blocks until context cancelled or fatal error.
func Start(ctx context.Context, cfg Config) error {
	abs, err := filepath.Abs(cfg.FilePath)
	if err != nil {
		return err
	}
	st, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.WriteFile(abs, []byte("# Document\n\n"), 0o644); err != nil {
				return err
			}
			st, err = os.Stat(abs)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if st.IsDir() {
		rootDir := filepath.Clean(abs)
		initial, err := PickInitialMarkdown(rootDir)
		if err != nil {
			return err
		}
		abs = filepath.Clean(initial)
		cfg.FilePath = abs
		cfg.ProjectRoot = rootDir
	} else {
		cfg.FilePath = abs
		rootDir := filepath.Dir(abs)
		rootDir, err = filepath.Abs(rootDir)
		if err != nil {
			return err
		}
		cfg.ProjectRoot = rootDir
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	content := string(raw)

	snapStore, err := NewSnapshotStore(abs)
	if err != nil {
		return err
	}

	threads, err := LoadThreads(abs)
	if err != nil {
		return err
	}
	threads = ReanchorThreadsWithStore(content, threads, snapStore)
	if err := SaveThreads(abs, threads); err != nil {
		return err
	}

	app := &App{
		cfg:       cfg,
		content:   content,
		threads:   threads,
		snapshots: snapStore,
		clients:   make(map[*websocket.Conn]struct{}),
	}

	app.broadcast = func(ev WSEvent) {
		app.wsMu.Lock()
		defer app.wsMu.Unlock()
		b, _ := json.Marshal(ev)
		for c := range app.clients {
			_ = c.WriteMessage(websocket.TextMessage, b)
		}
	}

	// Seed snapshot if history empty
	list, _ := app.snapshots.List()
	if len(list) == 0 {
		id, err := app.snapshots.WriteSnapshotNow(content)
		if err == nil {
			log.Printf("seed snapshot %s", id)
		}
	}

	stopWatch, err := WatchFile(abs, 150*time.Millisecond, func(newContent string) {
		app.mu.Lock()
		activePath := app.cfg.FilePath
		app.content = newContent
		app.threads = ReanchorThreadsWithStore(newContent, app.threads, app.snapshots)
		_ = SaveThreads(activePath, app.threads)
		app.mu.Unlock()
		app.broadcast(WSEvent{Type: "file_update", Content: newContent})
		app.broadcast(WSEvent{Type: "threads_update", Threads: app.snapshotThreads()})
		app.snapshots.ScheduleSnapshot(newContent, func(id string) {
			app.broadcast(WSEvent{Type: "snapshot_saved", SnapshotID: id, Timestamp: id})
		})
	})
	if err != nil {
		return err
	}
	app.stopWatch = stopWatch

	r := chi.NewRouter()

	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		return err
	}
	fsServer := http.FileServer(http.FS(static))
	r.Handle("/static/*", http.StripPrefix("/static/", fsServer))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		b, err := staticFS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})

	r.Get("/ws", app.handleWS)

	r.Get("/api/file-meta", app.handleFileMeta)
	r.Get("/api/project/tree", app.handleProjectTree)
	r.Post("/api/file/select", app.handleFileSelect)
	r.Post("/api/threads/upsert", app.handleUpsert)
	r.Post("/api/threads/reply", app.handleReply)
	r.Post("/api/threads/resolve", app.handleResolve)
	r.Post("/api/threads/delete", app.handleDeleteThread)
	r.Post("/api/threads/detached", app.handleDetached)
	r.Get("/api/snapshots", app.handleSnapshotsList)
	r.Get("/api/snapshots/diff", app.handleSnapshotsDiff)
	r.Get("/api/snapshots/{id}", app.handleSnapshotGet)
	r.Post("/api/snapshots/apply", app.handleSnapshotsApply)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
		if app.stopWatch != nil {
			app.stopWatch()
		}
	}()

	log.Printf("agentmark serving %s on http://127.0.0.1:%s", abs, cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (a *App) snapshotThreads() []CommentThread {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]CommentThread, len(a.threads))
	copy(out, a.threads)
	return out
}

func (a *App) handleWS(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	a.wsMu.Lock()
	a.clients[c] = struct{}{}
	a.wsMu.Unlock()

	a.mu.Lock()
	path := a.cfg.FilePath
	root := a.cfg.ProjectRoot
	relSlash := ""
	if root != "" && path != "" {
		if rel, err := filepath.Rel(root, path); err == nil {
			relSlash = filepath.ToSlash(rel)
		}
	}
	init := []WSEvent{
		{Type: "active_file", Name: filepath.Base(path), Path: path, RelPath: relSlash},
		{Type: "file_update", Content: a.content},
		{Type: "threads_update", Threads: a.threads},
	}
	a.mu.Unlock()
	for _, ev := range init {
		b, _ := json.Marshal(ev)
		_ = c.WriteMessage(websocket.TextMessage, b)
	}

	go func() {
		defer func() {
			a.wsMu.Lock()
			delete(a.clients, c)
			a.wsMu.Unlock()
			_ = c.Close()
		}()
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (a *App) handleUpsert(w http.ResponseWriter, r *http.Request) {
	var req UpsertThreadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.ID == "" || strings.TrimSpace(req.AnchorText) == "" {
		http.Error(w, "id and anchorText required", 400)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	idx := -1
	for i := range a.threads {
		if a.threads[i].ID == req.ID {
			idx = i
			break
		}
	}
	if idx >= 0 {
		a.threads[idx].AnchorText = req.AnchorText
		if req.Anchor != nil {
			hint := req.Anchor.StartOffset
			if hint < 0 {
				hint = strings.Index(a.content, req.AnchorText)
			}
			if hint >= 0 {
				end := hint + len(strings.TrimSpace(req.AnchorText))
				an := BuildAnchor(a.content, hint, end)
				a.threads[idx].Anchor = &an
			}
		}
		if req.Message != "" {
			a.threads[idx].Thread = append(a.threads[idx].Thread, CommentMessage{
				Role: "user", Body: req.Message, TS: time.Now().UnixMilli(),
			})
		}
	} else {
		t := CommentThread{ID: req.ID, AnchorText: req.AnchorText, Thread: nil}
		if req.Anchor != nil {
			hint := req.Anchor.StartOffset
			if hint < 0 {
				hint = strings.Index(a.content, req.AnchorText)
			}
			if hint >= 0 {
				end := hint + len(strings.TrimSpace(req.AnchorText))
				an := BuildAnchor(a.content, hint, end)
				t.Anchor = &an
			}
		}
		if req.Message != "" {
			t.Thread = append(t.Thread, CommentMessage{
				Role: "user", Body: req.Message, TS: time.Now().UnixMilli(),
			})
		}
		a.threads = append(a.threads, t)
	}
	a.threads = ReanchorThreadsWithStore(a.content, a.threads, a.snapshots)
	_ = SaveThreads(a.cfg.FilePath, a.threads)
	a.broadcast(WSEvent{Type: "threads_update", Threads: a.threads})
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (a *App) handleReply(w http.ResponseWriter, r *http.Request) {
	var req ThreadReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.ID == "" || strings.TrimSpace(req.Body) == "" {
		http.Error(w, "id and body required", 400)
		return
	}
	a.mu.Lock()
	found := false
	for i := range a.threads {
		if a.threads[i].ID == req.ID {
			a.threads[i].Thread = append(a.threads[i].Thread, CommentMessage{
				Role: "user", Body: req.Body, TS: time.Now().UnixMilli(),
			})
			found = true
			break
		}
	}
	if !found {
		a.mu.Unlock()
		http.Error(w, "thread not found", 404)
		return
	}
	_ = SaveThreads(a.cfg.FilePath, a.threads)
	t := a.threads
	a.mu.Unlock()
	a.broadcast(WSEvent{Type: "threads_update", Threads: t})
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (a *App) handleFileMeta(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	path := a.cfg.FilePath
	root := a.cfg.ProjectRoot
	a.mu.Unlock()
	relSlash := ""
	if root != "" && path != "" {
		if rel, err := filepath.Rel(root, path); err == nil {
			relSlash = filepath.ToSlash(rel)
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]string{
		"name":    filepath.Base(path),
		"path":    path,
		"relPath": relSlash,
	})
}

func (a *App) handleDeleteThread(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.ID == "" {
		http.Error(w, "id required", 400)
		return
	}
	a.mu.Lock()
	found := false
	next := make([]CommentThread, 0, len(a.threads))
	for _, t := range a.threads {
		if t.ID == req.ID {
			found = true
			continue
		}
		next = append(next, t)
	}
	if !found {
		a.mu.Unlock()
		http.Error(w, "thread not found", 404)
		return
	}
	a.threads = next
	_ = SaveThreads(a.cfg.FilePath, a.threads)
	t := a.threads
	a.mu.Unlock()
	a.broadcast(WSEvent{Type: "threads_update", Threads: t})
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (a *App) handleResolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string `json:"id"`
		Resolved bool   `json:"resolved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.ID == "" {
		http.Error(w, "id required", 400)
		return
	}
	a.mu.Lock()
	found := false
	for i := range a.threads {
		if a.threads[i].ID == req.ID {
			a.threads[i].Resolved = req.Resolved
			found = true
			break
		}
	}
	if !found {
		a.mu.Unlock()
		http.Error(w, "thread not found", 404)
		return
	}
	_ = SaveThreads(a.cfg.FilePath, a.threads)
	t := a.threads
	a.mu.Unlock()
	a.broadcast(WSEvent{Type: "threads_update", Threads: t})
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (a *App) handleDetached(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	a.threads = ReanchorThreadsWithStore(a.content, a.threads, a.snapshots)
	_ = SaveThreads(a.cfg.FilePath, a.threads)
	t := a.threads
	a.mu.Unlock()
	a.broadcast(WSEvent{Type: "threads_update", Threads: t})
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (a *App) handleSnapshotsList(w http.ResponseWriter, r *http.Request) {
	meta, err := a.snapshots.List()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	_ = json.NewEncoder(w).Encode(meta)
}

func (a *App) handleSnapshotGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	text, err := a.snapshots.Read(id)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"content": text})
}

func (a *App) handleSnapshotsDiff(w http.ResponseWriter, r *http.Request) {
	left := r.URL.Query().Get("a")
	right := r.URL.Query().Get("b")
	if left == "" || right == "" {
		http.Error(w, "a and b required", 400)
		return
	}
	hunks, err := a.snapshots.Diff(left, right)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_ = json.NewEncoder(w).Encode(DiffResponse{Hunks: hunks})
}

func (a *App) handleSnapshotsApply(w http.ResponseWriter, r *http.Request) {
	var req ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.LeftID == "" || req.RightID == "" {
		http.Error(w, "leftId and rightId required", 400)
		return
	}
	hunks, err := a.snapshots.Diff(req.LeftID, req.RightID)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	accept := make(map[string]struct{})
	for _, id := range req.AcceptedHunkIDs {
		accept[id] = struct{}{}
	}
	a.mu.Lock()
	current := a.content
	a.mu.Unlock()
	next, err := ApplyHunksToCurrent(current, hunks, accept)
	if err != nil {
		http.Error(w, err.Error(), 422)
		return
	}
	if err := os.WriteFile(a.cfg.FilePath, []byte(next), 0o644); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	a.mu.Lock()
	a.content = next
	a.threads = ReanchorThreadsWithStore(next, a.threads, a.snapshots)
	_ = SaveThreads(a.cfg.FilePath, a.threads)
	a.mu.Unlock()
	a.broadcast(WSEvent{Type: "file_update", Content: next})
	a.broadcast(WSEvent{Type: "threads_update", Threads: a.snapshotThreads()})
	id, _ := a.snapshots.WriteSnapshotNow(next)
	a.broadcast(WSEvent{Type: "snapshot_saved", SnapshotID: id, Timestamp: id})
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "snapshotId": id})
}
