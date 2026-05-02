package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	maxProjectMarkdownFiles = 2000
	maxWalkDepth            = 64
)

// PickInitialMarkdown chooses a markdown file to open when the user passes a
// project directory: README.md (any case) if present at the root, else the
// first root-level .md lexicographically, else the first .md from a recursive
// walk (same order as the sidebar). If none exist, creates README.md.
func PickInitialMarkdown(projectRoot string) (string, error) {
	projectRoot = filepath.Clean(projectRoot)
	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		return "", err
	}
	var rootMD []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.EqualFold(filepath.Ext(name), ".md") {
			rootMD = append(rootMD, name)
		}
	}
	sort.Strings(rootMD)
	for _, name := range rootMD {
		if strings.EqualFold(name, "README.md") {
			return filepath.Join(projectRoot, name), nil
		}
	}
	if len(rootMD) > 0 {
		return filepath.Join(projectRoot, rootMD[0]), nil
	}
	rels, err := collectMarkdownRelPaths(projectRoot)
	if err != nil {
		return "", err
	}
	if len(rels) > 0 {
		return filepath.Join(projectRoot, rels[0]), nil
	}
	p := filepath.Join(projectRoot, "README.md")
	if err := os.WriteFile(p, []byte("# Document\n\n"), 0o644); err != nil {
		return "", err
	}
	return p, nil
}

func collectMarkdownRelPaths(root string) ([]string, error) {
	var out []string
	var walk func(dir string, depth int) error
	walk = func(dir string, depth int) error {
		if depth > maxWalkDepth || len(out) >= maxProjectMarkdownFiles {
			return nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if len(out) >= maxProjectMarkdownFiles {
				return nil
			}
			name := e.Name()
			if e.IsDir() {
				if name != "." && strings.HasPrefix(name, ".") {
					continue
				}
				if err := walk(filepath.Join(dir, name), depth+1); err != nil {
					return err
				}
				continue
			}
			if !strings.EqualFold(filepath.Ext(name), ".md") {
				continue
			}
			full := filepath.Join(dir, name)
			rel, err := filepath.Rel(root, full)
			if err != nil {
				continue
			}
			out = append(out, rel)
		}
		return nil
	}
	if err := walk(root, 0); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func buildProjectTree(root string) (ProjectTreeResponse, error) {
	root = filepath.Clean(root)
	rels, err := collectMarkdownRelPaths(root)
	if err != nil {
		return ProjectTreeResponse{}, err
	}
	byLabel := map[string][]ProjectTreeFile{}
	for _, rel := range rels {
		relSlash := filepath.ToSlash(rel)
		parts := strings.Split(relSlash, "/")
		var label string
		if len(parts) == 1 {
			label = ""
		} else {
			label = parts[0]
		}
		abs := filepath.Join(root, rel)
		n, _ := OpenUnresolvedThreadCount(abs)
		byLabel[label] = append(byLabel[label], ProjectTreeFile{
			RelPath:          relSlash,
			Name:             parts[len(parts)-1],
			OpenCommentCount: n,
		})
	}
	var labels []string
	for k := range byLabel {
		labels = append(labels, k)
	}
	sort.Slice(labels, func(i, j int) bool {
		if labels[i] == "" {
			return true
		}
		if labels[j] == "" {
			return false
		}
		return labels[i] < labels[j]
	})
	sections := make([]ProjectTreeSection, 0, len(labels))
	for _, label := range labels {
		files := byLabel[label]
		sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
		sections = append(sections, ProjectTreeSection{Label: label, Files: files})
	}
	return ProjectTreeResponse{Root: root, Sections: sections}, nil
}

func (a *App) handleProjectTree(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	root := a.cfg.ProjectRoot
	a.mu.Unlock()
	if root == "" {
		http.Error(w, "project root not set", 500)
		return
	}
	tree, err := buildProjectTree(root)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	_ = json.NewEncoder(w).Encode(tree)
}

func (a *App) handleFileSelect(w http.ResponseWriter, r *http.Request) {
	var req SelectFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		http.Error(w, "path required", 400)
		return
	}
	a.mu.Lock()
	root := a.cfg.ProjectRoot
	a.mu.Unlock()
	if root == "" {
		http.Error(w, "project root not set", 500)
		return
	}
	abs, err := SafeJoin(root, req.Path)
	if err != nil {
		http.Error(w, "invalid path", 400)
		return
	}
	if !strings.EqualFold(filepath.Ext(abs), ".md") {
		http.Error(w, "not a markdown file", 400)
		return
	}
	st, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "not found", 404)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	if st.IsDir() {
		http.Error(w, "not a file", 400)
		return
	}
	if err := a.switchToFile(abs); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	a.mu.Lock()
	path := a.cfg.FilePath
	projRoot := a.cfg.ProjectRoot
	name := filepath.Base(path)
	content := a.content
	threads := make([]CommentThread, len(a.threads))
	copy(threads, a.threads)
	a.mu.Unlock()

	relSlash := ""
	if projRoot != "" && path != "" {
		if rel, err := filepath.Rel(projRoot, path); err == nil {
			relSlash = filepath.ToSlash(rel)
		}
	}
	a.broadcast(WSEvent{Type: "active_file", Name: name, Path: path, RelPath: relSlash})
	a.broadcast(WSEvent{Type: "file_update", Content: content})
	a.broadcast(WSEvent{Type: "threads_update", Threads: threads})
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// switchToFile replaces the active document, threads store, snapshot store, and file watcher.
func (a *App) switchToFile(abs string) error {
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

	a.mu.Lock()
	if a.stopWatch != nil {
		a.stopWatch()
		a.stopWatch = nil
	}
	if a.stopCommentsWatch != nil {
		a.stopCommentsWatch()
		a.stopCommentsWatch = nil
	}
	a.cfg.FilePath = abs
	a.content = content
	a.threads = threads
	a.snapshots = snapStore
	if err := SaveThreads(abs, threads); err != nil {
		a.mu.Unlock()
		return err
	}
	stopWatch, werr := WatchFile(abs, 150*time.Millisecond, func(newContent string) {
		a.mu.Lock()
		activePath := a.cfg.FilePath
		a.content = newContent
		a.threads = ReanchorThreadsWithStore(newContent, a.threads, a.snapshots)
		_ = SaveThreads(activePath, a.threads)
		a.mu.Unlock()
		a.broadcast(WSEvent{Type: "file_update", Content: newContent})
		a.broadcast(WSEvent{Type: "threads_update", Threads: a.snapshotThreads()})
		a.snapshots.ScheduleSnapshot(newContent, func(id string) {
			a.broadcast(WSEvent{Type: "snapshot_saved", SnapshotID: id, Timestamp: id})
		})
	})
	if werr != nil {
		a.mu.Unlock()
		return werr
	}
	a.stopWatch = stopWatch
	stopCommentsWatch, cerr := WatchFile(CommentsPathFor(abs), 150*time.Millisecond, func(_ string) {
		a.reloadThreadsFromSidecar()
	})
	if cerr != nil {
		a.stopWatch()
		a.stopWatch = nil
		a.mu.Unlock()
		return cerr
	}
	a.stopCommentsWatch = stopCommentsWatch
	a.mu.Unlock()

	list, _ := snapStore.List()
	if len(list) == 0 {
		if id, err := snapStore.WriteSnapshotNow(content); err == nil {
			log.Printf("seed snapshot %s", id)
		} else {
			log.Printf("seed snapshot: %v", err)
		}
	}
	return nil
}
