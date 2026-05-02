package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentmark/server"
)

func TestIntegrationCommentsLifecycle(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())

	root := t.TempDir()
	md := filepath.Join(root, "doc.md")
	writeFile(t, md, "# Title\n\nAlpha line\n\nBeta line\n")

	add := runCLI(t, []string{
		"comments", "add", md,
		"--anchor", "Alpha line",
		"--body", "Cursor\n\nPlease revise this.",
		"--role", "agent",
	})
	if add.err != nil {
		t.Fatalf("add error: %v stderr=%s", add.err, add.stderr)
	}
	addJSON := decodeJSONMap(t, add.stdout)
	if ok, _ := addJSON["ok"].(bool); !ok {
		t.Fatalf("expected ok add response: %s", add.stdout)
	}
	threadID, _ := addJSON["threadId"].(string)
	if threadID == "" {
		t.Fatalf("missing threadId: %s", add.stdout)
	}
	if _, err := os.Stat(server.CommentsPathFor(md)); err != nil {
		t.Fatalf("sidecar not created: %v", err)
	}

	reply := runCLI(t, []string{
		"comments", "reply", md,
		"--thread", threadID,
		"--body", "Cursor\n\nAck.",
		"--role", "agent",
	})
	if reply.err != nil {
		t.Fatalf("reply error: %v stderr=%s", reply.err, reply.stderr)
	}

	resolve := runCLI(t, []string{
		"comments", "resolve", md,
		"--thread", threadID,
	})
	if resolve.err != nil {
		t.Fatalf("resolve error: %v stderr=%s", resolve.err, resolve.stderr)
	}

	listResolved := runCLI(t, []string{
		"comments", "list", md,
		"--resolved",
	})
	if listResolved.err != nil {
		t.Fatalf("list resolved error: %v stderr=%s", listResolved.err, listResolved.stderr)
	}
	resolvedJSON := decodeJSONMap(t, listResolved.stdout)
	threads, _ := resolvedJSON["threads"].([]any)
	if len(threads) != 1 {
		t.Fatalf("expected 1 resolved thread, got %d body=%s", len(threads), listResolved.stdout)
	}

	unresolve := runCLI(t, []string{
		"comments", "unresolve", md,
		"--thread", threadID,
	})
	if unresolve.err != nil {
		t.Fatalf("unresolve error: %v stderr=%s", unresolve.err, unresolve.stderr)
	}

	listOpen := runCLI(t, []string{
		"comments", "list", md,
		"--open",
	})
	if listOpen.err != nil {
		t.Fatalf("list open error: %v stderr=%s", listOpen.err, listOpen.stderr)
	}
	openJSON := decodeJSONMap(t, listOpen.stdout)
	openThreads, _ := openJSON["threads"].([]any)
	if len(openThreads) != 1 {
		t.Fatalf("expected 1 open thread, got %d body=%s", len(openThreads), listOpen.stdout)
	}
}

func TestIntegrationCommentsListDirectory(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())

	root := t.TempDir()
	md1 := filepath.Join(root, "a.md")
	md2 := filepath.Join(root, "docs", "b.md")
	writeFile(t, md1, "# A\n\nAnchor one\n")
	writeFile(t, md2, "# B\n\nAnchor two\n")

	r1 := runCLI(t, []string{"comments", "add", md1, "--anchor", "Anchor one", "--body", "Cursor\n\none", "--role", "agent"})
	if r1.err != nil {
		t.Fatalf("add md1: %v stderr=%s", r1.err, r1.stderr)
	}
	r2 := runCLI(t, []string{"comments", "add", md2, "--anchor", "Anchor two", "--body", "Cursor\n\ntwo", "--role", "agent"})
	if r2.err != nil {
		t.Fatalf("add md2: %v stderr=%s", r2.err, r2.stderr)
	}

	list := runCLI(t, []string{"comments", "list", root})
	if list.err != nil {
		t.Fatalf("list dir: %v stderr=%s", list.err, list.stderr)
	}
	obj := decodeJSONMap(t, list.stdout)
	files, _ := obj["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d body=%s", len(files), list.stdout)
	}
}

func TestIntegrationCommentsValidationErrors(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	root := t.TempDir()
	md := filepath.Join(root, "doc.md")
	writeFile(t, md, "# Title\n\nAlpha line\n")

	cases := []struct {
		name        string
		args        []string
		wantErrLike string
	}{
		{
			name:        "add missing anchor",
			args:        []string{"comments", "add", md, "--body", "Cursor\n\nx", "--role", "agent"},
			wantErrLike: "--anchor and --body are required",
		},
		{
			name:        "add invalid agent intro",
			args:        []string{"comments", "add", md, "--anchor", "Alpha line", "--body", "\n\nx", "--role", "agent"},
			wantErrLike: "first line of --body must identify the source",
		},
		{
			name:        "add rejects missing exact anchor substring",
			args:        []string{"comments", "add", md, "--anchor", "Alpha `line`", "--body", "Cursor\n\nx", "--role", "agent"},
			wantErrLike: "anchor text not found exactly in file",
		},
		{
			name:        "reply thread missing",
			args:        []string{"comments", "reply", md, "--thread", "missing", "--body", "Cursor\n\nx", "--role", "agent"},
			wantErrLike: "thread not found",
		},
		{
			name:        "resolve missing thread flag",
			args:        []string{"comments", "resolve", md},
			wantErrLike: "--thread is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runCLI(t, tc.args)
			if got.err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
			if !hasJSONError(got.stderr) {
				t.Fatalf("expected JSON stderr, got %q", got.stderr)
			}
			if !strings.Contains(got.stderr, tc.wantErrLike) {
				t.Fatalf("stderr %q missing %q", got.stderr, tc.wantErrLike)
			}
		})
	}
}

func TestIntegrationCommentsAddDoesNotCreateDetachedSidecarEntry(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	root := t.TempDir()
	md := filepath.Join(root, "doc.md")
	writeFile(t, md, "# Title\n\nOptional: `--port 4173` (default)\n")

	got := runCLI(t, []string{
		"comments", "add", md,
		"--anchor", "Optional:  (default)",
		"--body", "Cursor\n\nx",
		"--role", "agent",
	})
	if got.err == nil {
		t.Fatalf("expected add failure for non-exact anchor")
	}
	if !hasJSONError(got.stderr) {
		t.Fatalf("expected JSON stderr, got %q", got.stderr)
	}
	if !strings.Contains(got.stderr, "anchor text not found exactly in file") {
		t.Fatalf("stderr %q missing anchor-not-found message", got.stderr)
	}

	sidecarPath := server.CommentsPathFor(md)
	if _, err := os.Stat(sidecarPath); err == nil {
		t.Fatalf("expected no sidecar created on failed add, found %s", sidecarPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat sidecar: %v", err)
	}
}

func TestIntegrationCommentsAgentIntroValidation(t *testing.T) {
	t.Setenv("AGENTMARK_DATA_DIR", t.TempDir())
	root := t.TempDir()
	md := filepath.Join(root, "doc.md")
	writeFile(t, md, "# Title\n\nAlpha line\n")

	addOK := runCLI(t, []string{
		"comments", "add", md,
		"--anchor", "Alpha line",
		"--body", "Cursor\n\nInitial note.",
		"--role", "agent",
	})
	if addOK.err != nil {
		t.Fatalf("seed add error: %v stderr=%s", addOK.err, addOK.stderr)
	}
	threadID, _ := decodeJSONMap(t, addOK.stdout)["threadId"].(string)
	if threadID == "" {
		t.Fatalf("missing threadId from seed add: %s", addOK.stdout)
	}

	tests := []struct {
		name        string
		args        []string
		wantErrLike string
	}{
		{
			name: "add rejects empty first line for agent role",
			args: []string{
				"comments", "add", md,
				"--anchor", "Alpha line",
				"--body", "\n\nbody",
				"--role", "agent",
			},
			wantErrLike: "Got: empty first line.",
		},
		{
			name: "add rejects overly long first line for agent role",
			args: []string{
				"comments", "add", md,
				"--anchor", "Alpha line",
				"--body", "this first line is intentionally longer than forty chars\n\nbody",
				"--role", "agent",
			},
			wantErrLike: "Limit: 40 chars on a single line.",
		},
		{
			name: "reply rejects empty first line for agent role",
			args: []string{
				"comments", "reply", md,
				"--thread", threadID,
				"--body", "\n\nreply",
				"--role", "agent",
			},
			wantErrLike: "Got: empty first line.",
		},
		{
			name: "reply rejects overly long first line for agent role",
			args: []string{
				"comments", "reply", md,
				"--thread", threadID,
				"--body", "this first line is intentionally longer than forty chars\n\nreply",
				"--role", "agent",
			},
			wantErrLike: "Limit: 40 chars on a single line.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runCLI(t, tc.args)
			if got.err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
			if !hasJSONError(got.stderr) {
				t.Fatalf("expected JSON stderr, got %q", got.stderr)
			}
			if !strings.Contains(got.stderr, tc.wantErrLike) {
				t.Fatalf("stderr %q missing %q", got.stderr, tc.wantErrLike)
			}
		})
	}

	t.Run("add allows empty first line for user role", func(t *testing.T) {
		got := runCLI(t, []string{
			"comments", "add", md,
			"--anchor", "Alpha line",
			"--body", "\n\nhuman message body",
			"--role", "user",
		})
		if got.err != nil {
			t.Fatalf("expected success for role=user: %v stderr=%s", got.err, got.stderr)
		}
	})

	t.Run("reply allows empty first line for user role", func(t *testing.T) {
		got := runCLI(t, []string{
			"comments", "reply", md,
			"--thread", threadID,
			"--body", "\n\nhuman reply body",
			"--role", "user",
		})
		if got.err != nil {
			t.Fatalf("expected success for role=user: %v stderr=%s", got.err, got.stderr)
		}
	})
}
