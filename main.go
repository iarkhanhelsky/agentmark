package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"agentmark/server"
)

func printHelp(w io.Writer) {
	fmt.Fprintf(w, `AgentMark — local-first markdown review: preview, anchored comment threads (sidecar JSON), and file snapshot history.

Usage:
  %[1]s [flags] <path>
  %[1]s help

Arguments:
  path    Markdown file or directory to watch and serve at http://127.0.0.1:<port>/

Flags:
`, os.Args[0])
	flag.CommandLine.SetOutput(w)
	flag.PrintDefaults()
	fmt.Fprintf(w, `
Examples:
  %[1]s ./README.md
  %[1]s --port 8080 --no-open ./docs/

Comments are stored in .<filename>.comments.json next to the document.
Snapshot history is kept under the OS app data directory (see README).
`, os.Args[0])
}

func main() {
	port := flag.String("port", "4173", "HTTP listen port for the review UI")
	noOpen := flag.Bool("no-open", false, "do not open a browser tab")

	flag.Usage = func() {
		printHelp(os.Stderr)
	}

	// Subcommand-style help before flag parsing (supports "agentmark help").
	if len(os.Args) >= 2 && os.Args[1] == "help" {
		printHelp(os.Stdout)
		return
	}

	flag.Parse()
	args := flag.Args()
	if len(args) == 1 && args[0] == "help" {
		printHelp(os.Stdout)
		return
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "agentmark: need exactly one path (markdown file or directory), or run \"agentmark help\".")
		printHelp(os.Stderr)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if !*noOpen {
		go func() {
			time.Sleep(400 * time.Millisecond)
			openBrowser("http://127.0.0.1:" + *port)
		}()
	}

	if err := server.Start(ctx, server.Config{FilePath: args[0], Port: *port}); err != nil {
		log.Fatal(err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
