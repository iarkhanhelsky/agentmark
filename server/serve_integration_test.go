package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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
	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get / status=%d body=%s", resp.StatusCode, string(body))
	}
	host, _ := os.Hostname()
	if strings.TrimSpace(host) == "" {
		host = "localhost"
	}
	wantServerID := html.EscapeString(host + ":" + root)
	if !strings.Contains(string(body), `meta name="agentmark-server-id"`) {
		t.Fatalf("index missing agentmark-server-id meta: %s", string(body))
	}
	if !strings.Contains(string(body), `content="`+wantServerID+`"`) {
		t.Fatalf("index meta content mismatch, want content=%q", wantServerID)
	}
	if strings.Contains(string(body), "Copy context") {
		t.Fatalf("index should not include removed Copy context action")
	}

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

func TestIntegrationServeDoesNotCreateEmptySidecarOnStartup(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	root := t.TempDir()
	file := filepath.Join(root, "doc.md")
	if err := os.WriteFile(file, []byte("# Doc\n\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	sidecar := CommentsPathFor(file)
	if _, err := os.Stat(sidecar); err == nil {
		t.Fatalf("precondition: sidecar should not exist yet")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat sidecar: %v", err)
	}

	port := freeTCPPort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Start(ctx, Config{FilePath: file, Port: port})
	}()
	waitForReady(t, baseURL)

	if _, err := os.Stat(sidecar); err == nil {
		t.Fatalf("expected no sidecar for uncommented file after startup, found %s", sidecar)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat sidecar: %v", err)
	}

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

func TestIntegrationServeDoesNotCreateEmptySidecarOnFileSwitch(t *testing.T) {
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
	other := filepath.Join(docs, "other.md")
	if err := os.WriteFile(other, []byte("# Other\n\n"), 0o644); err != nil {
		t.Fatalf("write other: %v", err)
	}
	otherSidecar := CommentsPathFor(other)
	if _, err := os.Stat(otherSidecar); err == nil {
		t.Fatalf("precondition: other sidecar should not exist")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat: %v", err)
	}

	port := freeTCPPort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Start(ctx, Config{FilePath: root, Port: port})
	}()
	waitForReady(t, baseURL)

	postJSONExpectStatus(t, baseURL+"/api/file/select", map[string]any{
		"path": "docs/other.md",
	}, http.StatusOK)

	if _, err := os.Stat(otherSidecar); err == nil {
		t.Fatalf("expected no sidecar after switching to uncommented file, found %s", otherSidecar)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat sidecar: %v", err)
	}

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

func TestIntegrationServeRewritesExistingEmptySidecar(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	root := t.TempDir()
	file := filepath.Join(root, "doc.md")
	if err := os.WriteFile(file, []byte("# Doc\n\nBody\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	sidecar := CommentsPathFor(file)
	emptyJSON := `{"version":1,"threads":[]}` + "\n"
	if err := os.WriteFile(sidecar, []byte(emptyJSON), 0o644); err != nil {
		t.Fatalf("write empty sidecar: %v", err)
	}

	port := freeTCPPort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Start(ctx, Config{FilePath: file, Port: port})
	}()
	waitForReady(t, baseURL)

	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("expected existing empty sidecar to remain after startup: %v", err)
	}

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

func TestIntegrationUpsertThreadWithoutMessageStoresEmptyThreadSlice(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	root := t.TempDir()
	file := filepath.Join(root, "doc.md")
	if err := os.WriteFile(file, []byte("# Doc\n\nHello\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	port := freeTCPPort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Start(ctx, Config{FilePath: file, Port: port})
	}()
	waitForReady(t, baseURL)

	postJSONExpectStatus(t, baseURL+"/api/threads/upsert", map[string]any{
		"id":         "empty-msg-thread",
		"anchorText": "Hello",
	}, http.StatusOK)

	sidecar := filepath.Join(root, ".doc.md.comments.json")
	raw, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	if strings.Contains(string(raw), `"thread":null`) {
		t.Fatalf("sidecar should serialize thread as empty array, not null: %s", string(raw))
	}
	var cf CommentsFile
	if err := json.Unmarshal(raw, &cf); err != nil {
		t.Fatalf("unmarshal sidecar: %v", err)
	}
	if len(cf.Threads) != 1 {
		t.Fatalf("threads count = %d, want 1", len(cf.Threads))
	}
	if cf.Threads[0].Thread == nil {
		t.Fatal("decoded Thread slice is nil")
	}
	if len(cf.Threads[0].Thread) != 0 {
		t.Fatalf("message count = %d, want 0", len(cf.Threads[0].Thread))
	}

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

func TestIntegrationServePushesThreadsUpdateOnExternalSidecarWrite(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	root := t.TempDir()
	file := filepath.Join(root, "doc.md")
	if err := os.WriteFile(file, []byte("# Doc\n\nBody\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	port := freeTCPPort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)
	wsURL := fmt.Sprintf("ws://127.0.0.1:%s/ws", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Start(ctx, Config{FilePath: file, Port: port})
	}()
	waitForReady(t, baseURL)

	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))

	for i := 0; i < 3; i++ {
		if _, _, err := c.ReadMessage(); err != nil {
			t.Fatalf("read bootstrap ws message: %v", err)
		}
	}

	threads := []CommentThread{
		{
			ID:         "ext-1",
			AnchorText: "Body",
			Thread: []CommentMessage{
				{Role: "agent", Body: "External write", TS: time.Now().UnixMilli()},
			},
		},
	}
	if err := SaveThreads(file, threads); err != nil {
		t.Fatalf("save external sidecar: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		_ = c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, b, err := c.ReadMessage()
		if err != nil {
			continue
		}
		var ev WSEvent
		if err := json.Unmarshal(b, &ev); err != nil {
			continue
		}
		if ev.Type != "threads_update" {
			continue
		}
		for _, t := range ev.Threads {
			if t.ID == "ext-1" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("did not receive threads_update for external sidecar write")
	}
}

func TestIntegrationServeWSBootstrapLoadsLatestSidecar(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	root := t.TempDir()
	file := filepath.Join(root, "doc.md")
	if err := os.WriteFile(file, []byte("# Doc\n\nBody\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	port := freeTCPPort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)
	wsURL := fmt.Sprintf("ws://127.0.0.1:%s/ws", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Start(ctx, Config{FilePath: file, Port: port})
	}()
	waitForReady(t, baseURL)

	threads := []CommentThread{
		{
			ID:         "bootstrap-1",
			AnchorText: "Body",
			Thread: []CommentMessage{
				{Role: "agent", Body: "Latest from sidecar", TS: time.Now().UnixMilli()},
			},
		},
	}
	if err := SaveThreads(file, threads); err != nil {
		t.Fatalf("save sidecar: %v", err)
	}

	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))

	found := false
	for i := 0; i < 5; i++ {
		_, b, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read ws bootstrap message: %v", err)
		}
		var ev WSEvent
		if err := json.Unmarshal(b, &ev); err != nil {
			continue
		}
		if ev.Type != "threads_update" {
			continue
		}
		for _, t := range ev.Threads {
			if t.ID == "bootstrap-1" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("ws bootstrap did not include latest sidecar threads")
	}
}

func TestIntegrationServeWSBootstrapDedupesDuplicateThreadIDs(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	root := t.TempDir()
	file := filepath.Join(root, "doc.md")
	if err := os.WriteFile(file, []byte("# Doc\n\nBody\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	port := freeTCPPort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)
	wsURL := fmt.Sprintf("ws://127.0.0.1:%s/ws", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Start(ctx, Config{FilePath: file, Port: port})
	}()
	waitForReady(t, baseURL)

	threads := []CommentThread{
		{
			ID:         "dup-1",
			AnchorText: "missing text",
			Detached:   true,
			Thread: []CommentMessage{
				{Role: "agent", Body: "detached copy", TS: time.Now().UnixMilli()},
			},
		},
		{
			ID:         "dup-1",
			AnchorText: "Body",
			Thread: []CommentMessage{
				{Role: "agent", Body: "attached copy", TS: time.Now().UnixMilli() + 1},
			},
		},
	}
	if err := SaveThreads(file, threads); err != nil {
		t.Fatalf("save sidecar: %v", err)
	}

	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))

	var got []CommentThread
	for i := 0; i < 6; i++ {
		_, b, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read ws bootstrap message: %v", err)
		}
		var ev WSEvent
		if err := json.Unmarshal(b, &ev); err != nil {
			continue
		}
		if ev.Type == "threads_update" {
			got = ev.Threads
			break
		}
	}
	if len(got) == 0 {
		t.Fatal("ws bootstrap did not include threads_update")
	}
	dupCount := 0
	var canonical *CommentThread
	for i := range got {
		if got[i].ID != "dup-1" {
			continue
		}
		dupCount++
		canonical = &got[i]
	}
	if dupCount != 1 {
		t.Fatalf("expected exactly one dup-1 thread, got %d (threads=%+v)", dupCount, got)
	}
	if canonical == nil {
		t.Fatal("missing canonical dup-1 thread")
	}
	if canonical.Detached {
		t.Fatalf("expected canonical dup-1 thread to be attached, got detached: %+v", *canonical)
	}
}

func TestIntegrationServeWSBootstrapConcurrentBroadcasts(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	root := t.TempDir()
	file := filepath.Join(root, "doc.md")
	if err := os.WriteFile(file, []byte("# Doc\n\nBody\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	port := freeTCPPort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)
	wsURL := fmt.Sprintf("ws://127.0.0.1:%s/ws", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Start(ctx, Config{FilePath: file, Port: port})
	}()
	waitForReady(t, baseURL)

	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer c.Close()

	postErrCh := make(chan error, 1)
	go func() {
		for i := 0; i < 20; i++ {
			payload := map[string]any{
				"id":         fmt.Sprintf("race-%d", i),
				"anchorText": "Body",
				"message":    "race check",
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				postErrCh <- err
				return
			}
			resp, err := http.Post(baseURL+"/api/threads/upsert", "application/json", bytes.NewReader(raw))
			if err != nil {
				postErrCh <- err
				return
			}
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				postErrCh <- fmt.Errorf("upsert status=%d", resp.StatusCode)
				return
			}
		}
		postErrCh <- nil
	}()

	bootstrapSeen := 0
	threadsUpdateSeen := false
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		_ = c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		_, b, err := c.ReadMessage()
		if err != nil {
			continue
		}
		var ev WSEvent
		if err := json.Unmarshal(b, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "active_file", "file_update":
			bootstrapSeen++
		case "threads_update":
			threadsUpdateSeen = true
		}
		if bootstrapSeen >= 2 && threadsUpdateSeen {
			break
		}
	}

	if err := <-postErrCh; err != nil {
		t.Fatalf("post threads/upsert: %v", err)
	}
	if bootstrapSeen < 2 {
		t.Fatalf("expected bootstrap ws messages, got %d", bootstrapSeen)
	}
	if !threadsUpdateSeen {
		t.Fatal("did not receive threads_update under concurrent broadcast load")
	}
}
