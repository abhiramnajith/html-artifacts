// Command html-artifacts serves self-contained HTML artifacts and their
// annotations from a local directory, bound to 127.0.0.1 only.
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/abhiramnajith/html-artifacts/server/internal/server"
	"github.com/abhiramnajith/html-artifacts/server/internal/storage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "html-artifacts:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("no command given")
	}

	switch args[0] {
	case "serve":
		return serve(args[1:])
	case "-h", "--help", "help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fs.Int("port", 47600, "port to bind on 127.0.0.1")
	dir := fs.String("dir", defaultArtifactsDir(), "directory holding artifacts and annotations")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := os.MkdirAll(*dir, 0o700); err != nil { // 0700: Finding 4 — not readable by other local users
		return fmt.Errorf("create artifacts dir %s: %w", *dir, err)
	}

	srv, err := server.New(storage.New(*dir))
	if err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	addr := server.ListenAddr(*port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	httpSrv := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Printf("html-artifacts serving %s at http://%s/artifacts\n", *dir, addr)
	if err := httpSrv.Serve(ln); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

func defaultArtifactsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./artifacts"
	}
	return filepath.Join(home, ".html-artifacts", "artifacts")
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: html-artifacts serve [--port N] [--dir PATH]")
}
