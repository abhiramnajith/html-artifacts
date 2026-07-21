// Command html-artifacts serves self-contained HTML artifacts and their
// annotations from a local directory, bound to 127.0.0.1 only.
//
// Phase 0 ships a compiling skeleton: the `serve` subcommand parses flags and
// reports its configuration. The HTTP handlers and storage layer arrive in
// Phase 2.
package main

import (
	"flag"
	"fmt"
	"os"
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
	port := fs.Int("port", 7777, "port to bind on 127.0.0.1")
	dir := fs.String("dir", "./artifacts", "directory holding artifacts and annotations")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Phase 2 wires up the HTTP server against internal/server + internal/storage.
	fmt.Printf("serve: would bind 127.0.0.1:%d serving %s (not implemented until Phase 2)\n", *port, *dir)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: html-artifacts serve [--port N] [--dir PATH]")
}
