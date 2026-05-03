package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"agentmark/cli/internal/cmdcore"
)

// ErrAlreadyReported means stderr already contains a structured error (e.g. JSON).
var ErrAlreadyReported = cmdcore.ErrAlreadyReported

// WriteError writes a JSON error object to stderr.
func WriteError(err error) {
	cmdcore.WriteError(err)
}

// AbsMarkdown resolves path to absolute; must exist and must not be a directory.
func AbsMarkdown(path string) (string, error) {
	return cmdcore.AbsMarkdown(path)
}

// AbsExistingPath resolves path to absolute; must exist. isDir is true when path is a directory.
func AbsExistingPath(path string) (abs string, isDir bool, err error) {
	return cmdcore.AbsExistingPath(path)
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
