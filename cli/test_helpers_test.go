package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type cliResult struct {
	stdout string
	stderr string
	err    error
}

var stdioMu sync.Mutex

func runCLI(t *testing.T, args []string) cliResult {
	t.Helper()
	stdioMu.Lock()
	defer stdioMu.Unlock()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW

	cmd := newRootCmd()
	cmd.SetArgs(args)
	runErr := cmd.Execute()

	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	outBytes, _ := io.ReadAll(stdoutR)
	errBytes, _ := io.ReadAll(stderrR)
	_ = stdoutR.Close()
	_ = stderrR.Close()

	return cliResult{
		stdout: string(outBytes),
		stderr: string(errBytes),
		err:    runErr,
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func decodeJSONMap(t *testing.T, raw string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode json %q: %v", raw, err)
	}
	return out
}

func decodeJSONArrayMap(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode json array %q: %v", raw, err)
	}
	return out
}

func hasJSONError(raw string) bool {
	raw = strings.TrimSpace(raw)
	return strings.HasPrefix(raw, `{"error":`)
}
