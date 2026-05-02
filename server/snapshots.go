package server

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxSnapshots = 100

// SnapshotStore manages version history outside the workspace.
type SnapshotStore struct {
	mu           sync.Mutex
	filePathAbs  string
	dir          string
	lastContent  string
	pendingTimer *time.Timer
}

func dataDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	case "windows":
		a := os.Getenv("APPDATA")
		if a == "" {
			return "", fmt.Errorf("APPDATA not set")
		}
		return a, nil
	default:
		if d := os.Getenv("XDG_DATA_HOME"); d != "" {
			return d, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share"), nil
	}
}

func historyDirForFile(absFile string) (string, error) {
	base, err := dataDir()
	if err != nil {
		return "", err
	}
	h := sha1.Sum([]byte(absFile))
	key := hex.EncodeToString(h[:])
	return filepath.Join(base, "agentmark", "history", key), nil
}

// NewSnapshotStore creates a store for the given absolute markdown path.
func NewSnapshotStore(absFile string) (*SnapshotStore, error) {
	dir, err := historyDirForFile(absFile)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &SnapshotStore{filePathAbs: absFile, dir: dir}, nil
}

// Dir returns the history directory for this file.
func (s *SnapshotStore) Dir() string { return s.dir }

// List returns snapshot metadata newest-first.
func (s *SnapshotStore) List() ([]SnapshotMeta, error) {
	names, err := s.listNames()
	if err != nil {
		return nil, err
	}
	var out []SnapshotMeta
	for _, n := range names {
		id := strings.TrimSuffix(n, ".md")
		p := filepath.Join(s.dir, n)
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		text := string(raw)
		lines := 0
		if text != "" {
			lines = len(strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"))
		}
		out = append(out, SnapshotMeta{ID: id, TS: id, Lines: lines})
	}
	return out, nil
}

// Read returns snapshot content by id (filename stem).
func (s *SnapshotStore) Read(id string) (string, error) {
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid id")
	}
	p := filepath.Join(s.dir, id+".md")
	raw, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ScheduleSnapshot debounces snapshot writes (2s) when file content changes.
func (s *SnapshotStore) ScheduleSnapshot(content string, onSaved func(id string)) {
	s.mu.Lock()
	s.lastContent = content
	if s.pendingTimer != nil {
		s.pendingTimer.Stop()
	}
	s.pendingTimer = time.AfterFunc(2*time.Second, func() {
		s.mu.Lock()
		c := s.lastContent
		s.mu.Unlock()
		id, changed, err := s.writeIfChanged(c)
		if err == nil && changed && onSaved != nil {
			onSaved(id)
		}
	})
	s.mu.Unlock()
}

func snapshotIDNow() string {
	return time.Now().UTC().Format("2006-01-02T15-04-05.000000000Z")
}

// writeIfChanged returns (id, true) if a new snapshot file was written.
func (s *SnapshotStore) writeIfChanged(content string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	names, err := s.listNamesUnlocked()
	if err != nil {
		return "", false, err
	}
	if len(names) > 0 {
		lastPath := filepath.Join(s.dir, names[0])
		if raw, err := os.ReadFile(lastPath); err == nil && string(raw) == content {
			return strings.TrimSuffix(names[0], ".md"), false, nil
		}
	}
	id := snapshotIDNow()
	p := filepath.Join(s.dir, id+".md")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		return "", false, err
	}
	_ = s.pruneUnlocked()
	return id, true, nil
}

func (s *SnapshotStore) listNames() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listNamesUnlocked()
}

func (s *SnapshotStore) listNamesUnlocked() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".md") {
			names = append(names, n)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names, nil
}

func (s *SnapshotStore) pruneUnlocked() error {
	names, err := s.listNamesUnlocked()
	if err != nil {
		return err
	}
	for i := maxSnapshots; i < len(names); i++ {
		_ = os.Remove(filepath.Join(s.dir, names[i]))
	}
	return nil
}

// WriteSnapshotNow writes a snapshot immediately (after apply or startup seed).
func (s *SnapshotStore) WriteSnapshotNow(content string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := snapshotIDNow()
	p := filepath.Join(s.dir, id+".md")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		return "", err
	}
	_ = s.pruneUnlocked()
	return id, nil
}

// Diff loads two snapshots and returns hunks.
func (s *SnapshotStore) Diff(leftID, rightID string) ([]DiffHunk, error) {
	left, err := s.Read(leftID)
	if err != nil {
		return nil, err
	}
	right, err := s.Read(rightID)
	if err != nil {
		return nil, err
	}
	return LineDiff(left, right), nil
}
