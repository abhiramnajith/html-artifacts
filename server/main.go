// Command vellum serves self-contained HTML artifacts and their
// annotations from a local directory, bound to 127.0.0.1 only.
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	assets "github.com/abhiramnajith/vellum/server/embed"
	markdown "github.com/abhiramnajith/vellum/server/internal/markdown"
	"github.com/abhiramnajith/vellum/server/internal/server"
	"github.com/abhiramnajith/vellum/server/internal/storage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "vellum:", err)
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
	case "render":
		return renderCmd(args[1:])
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
	fmt.Printf("vellum serving %s at http://%s/artifacts\n", *dir, addr)
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
	return filepath.Join(home, ".vellum", "artifacts")
}

// haHome mirrors ensure-server.sh's HA_HOME resolution:
// ${VELLUM_HOME:-~/.vellum}
func haHome() string {
	if h := os.Getenv("VELLUM_HOME"); h != "" {
		return h
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".vellum")
	}
	return ".vellum"
}

// renderDefaultDir mirrors ensure-server.sh's DIR resolution:
// ${VELLUM_DIR:-$HA_HOME/artifacts}
func renderDefaultDir() string {
	if d := os.Getenv("VELLUM_DIR"); d != "" {
		return d
	}
	return filepath.Join(haHome(), "artifacts")
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func renderCmd(args []string) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	dir := fs.String("dir", renderDefaultDir(), "artifacts directory")
	title := fs.String("title", "", "artifact title (defaults to the file name)")
	idFlag := fs.String("id", "", "explicit artifact id (defaults to slug+timestamp)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: vellum render <path.md> [--title T] [--dir D] [--id ID]")
	}
	path := fs.Arg(0)
	md, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	name := *title
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	id := *idFlag
	if id == "" {
		id = slugify(name) + "-" + time.Now().Format("20060102-150405")
	}
	if !storage.ValidID(id) {
		return fmt.Errorf("invalid artifact id %q (must match ^[a-z0-9-]+$)", id)
	}

	tmpl, err := assets.Files.ReadFile("base.html")
	if err != nil {
		return fmt.Errorf("load template: %w", err)
	}
	now := time.Now()
	out := string(tmpl)
	repl := map[string]string{
		"{{TITLE}}":           name,
		"{{ARTIFACT_ID}}":     id,
		"{{GENERATED_HUMAN}}": now.Format("02 Jan 2006, 15:04"),
		"{{GENERATED_ISO}}":   now.Format("2006-01-02T15:04:05"),
		"{{CONTENT}}":         markdown.Render(string(md)),
	}
	for k, v := range repl {
		out = strings.ReplaceAll(out, k, v)
	}

	if err := os.MkdirAll(*dir, 0o700); err != nil {
		return fmt.Errorf("create artifacts dir: %w", err)
	}
	dest := filepath.Join(*dir, id+".html")
	if err := os.WriteFile(dest, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write artifact: %w", err)
	}

	fmt.Printf("rendered %s -> %s\n", path, dest)
	if p, err := os.ReadFile(filepath.Join(haHome(), "port")); err == nil {
		fmt.Printf("view: http://127.0.0.1:%s/view/%s\n", strings.TrimSpace(string(p)), id)
	} else {
		fmt.Printf("start the server (make serve) then open /view/%s\n", id)
	}
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: vellum serve [--port N] [--dir PATH]")
	fmt.Fprintln(os.Stderr, "       vellum render <path.md> [--title T] [--dir D] [--id ID]")
}
