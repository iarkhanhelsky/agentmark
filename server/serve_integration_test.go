package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIntegrationServeCoreAPI(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())

	root := t.TempDir()
	readme := filepath.Join(root, "README.md")
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(readme, []byte("# Root\n\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docs, "guide.md"), []byte("# Guide\n\n"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}

	port := freeTCPPort(t)
	baseURL := "http://127.0.0.1:" + port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Start(ctx, Config{
			FilePath: root,
			Port:     port,
		})
	}()

	waitForReady(t, baseURL)

	meta := getJSONMap(t, baseURL+"/api/file-meta")
	if got := meta["name"]; got != "README.md" {
		t.Fatalf("active file name = %v, want README.md", got)
	}
	if gotPath, _ := meta["path"].(string); !strings.HasSuffix(filepath.ToSlash(gotPath), "/README.md") {
		t.Fatalf("unexpected active path %q", gotPath)
	}
	if gotRel := meta["relPath"]; gotRel != "README.md" {
		t.Fatalf("relPath = %v, want README.md", gotRel)
	}

	treeRaw := getJSON(t, baseURL+"/api/project/tree")
	var tree ProjectTreeResponse
	if err := json.Unmarshal(treeRaw, &tree); err != nil {
		t.Fatalf("decode project tree: %v body=%s", err, string(treeRaw))
	}
	if filepath.Clean(tree.Root) != filepath.Clean(root) {
		t.Fatalf("tree root = %q want %q", tree.Root, root)
	}
	if len(tree.Sections) < 2 {
		t.Fatalf("expected at least 2 sections, got %#v", tree.Sections)
	}

	postJSONExpectStatus(t, baseURL+"/api/file/select", map[string]any{
		"path": "docs/guide.md",
	}, http.StatusOK)

	metaAfter := getJSONMap(t, baseURL+"/api/file-meta")
	if got := metaAfter["name"]; got != "guide.md" {
		t.Fatalf("after select active file name = %v, want guide.md", got)
	}
	if got := metaAfter["relPath"]; got != "docs/guide.md" {
		t.Fatalf("after select relPath = %v, want docs/guide.md", got)
	}

	postJSONExpectStatus(t, baseURL+"/api/file/select", map[string]any{
		"path": "../outside.md",
	}, http.StatusBadRequest)

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("server exited with error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop within timeout")
	}
}

func freeTCPPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	defer l.Close()
	_, p, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	return p
}

func waitForReady(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/api/file-meta")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(75 * time.Millisecond)
	}
	t.Fatalf("server not ready: %s", baseURL)
}

func getJSON(t *testing.T, url string) []byte {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
	return body
}

func getJSONMap(t *testing.T, url string) map[string]any {
	t.Helper()
	body := getJSON(t, url)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode map from %s: %v body=%s", url, err, string(body))
	}
	return out
}

func postJSONExpectStatus(t *testing.T, url string, payload any, wantStatus int) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		t.Fatalf("post %s status=%d want=%d body=%s", url, resp.StatusCode, wantStatus, string(body))
	}
	if wantStatus == http.StatusOK && !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("post %s missing ok=true body=%s", url, string(body))
	}
	if wantStatus != http.StatusOK && len(body) == 0 {
		t.Fatalf("post %s expected error body, got empty", url)
	}
}

func TestIntegrationServeStartupShutdown(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	file := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(file, []byte("# Doc\n\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	port := freeTCPPort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Start(ctx, Config{FilePath: file, Port: port})
	}()
	waitForReady(t, baseURL)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("start returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop on cancel")
	}
}
