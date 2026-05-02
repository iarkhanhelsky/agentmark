package server

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SafeJoin returns an absolute path under root for rel (relative or absolute).
// rel may use slash or native separators. Rejects paths that escape root.
func SafeJoin(root, rel string) (string, error) {
	root = filepath.Clean(root)
	if root == "" {
		return "", fmt.Errorf("empty root")
	}
	rel = strings.TrimSpace(rel)
	var joined string
	if filepath.IsAbs(rel) {
		joined = filepath.Clean(rel)
	} else {
		rel = filepath.Clean(filepath.FromSlash(rel))
		if rel == "." || rel == "" {
			joined = root
		} else {
			joined = filepath.Join(root, rel)
		}
	}
	joined = filepath.Clean(joined)
	relToRoot, err := filepath.Rel(root, joined)
	if err != nil {
		return "", err
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root")
	}
	return joined, nil
}
