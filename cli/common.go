package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrAlreadyReported means stderr already contains a structured error (e.g. JSON).
var ErrAlreadyReported = errors.New("error already written to stderr")

// WriteError writes a JSON error object to stderr.
func WriteError(err error) {
	enc := json.NewEncoder(os.Stderr)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(map[string]string{"error": err.Error()})
}

// AbsMarkdown resolves path to absolute; must exist and must not be a directory.
func AbsMarkdown(path string) (string, error) {
	abs, isDir, err := AbsExistingPath(path)
	if err != nil {
		return "", err
	}
	if isDir {
		return "", fmt.Errorf("path must be a markdown file, not a directory")
	}
	return abs, nil
}

// AbsExistingPath resolves path to absolute; must exist. isDir is true when path is a directory.
func AbsExistingPath(path string) (abs string, isDir bool, err error) {
	abs, err = filepath.Abs(path)
	if err != nil {
		return "", false, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", false, err
	}
	return abs, st.IsDir(), nil
}

// ExpandUser replaces leading ~ with home directory.
func ExpandUser(p string) (string, error) {
	if p == "" || p[0] != '~' {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if len(p) == 1 {
		return home, nil
	}
	if p[1] != '/' && p[1] != '\\' {
		return "", fmt.Errorf("unsupported ~ path")
	}
	return filepath.Join(home, p[2:]), nil
}
