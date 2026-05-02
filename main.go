package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"agentmark/server"
)

func main() {
	port := flag.String("port", "4173", "HTTP listen port")
	noOpen := flag.Bool("no-open", false, "do not open a browser tab")
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Fatal("usage: agentmark [--port=4173] [--no-open] <path-to-markdown.md>")
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
