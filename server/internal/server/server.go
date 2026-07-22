// Package server wires the HTTP handlers (view, list, editor shell) onto a
// storage.Store. It never inspects or branches on which agent produced or
// consumes an artifact — the contract is files and HTTP only.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	assets "github.com/abhiramnajith/vellum/server/embed"
	"github.com/abhiramnajith/vellum/server/internal/storage"
)

// shellTag is injected into every served artifact so the annotation editor
// loads around it without modifying the artifact file on disk.
const shellTag = `<script src="/_editor/shell.js" defer></script>`

// maxAnnotationBody caps the POST body for annotations. Localhost-only, but a
// bound keeps a runaway client from filling memory or disk.
const maxAnnotationBody = 1 << 20 // 1 MiB

// Annotation is one comment attached to an element or text range in an artifact.
type Annotation struct {
	ID           string `json:"id"`
	Selector     string `json:"selector"`
	SelectedText string `json:"selectedText"`
	Comment      string `json:"comment"`
	CreatedAt    string `json:"createdAt"`
}

// AnnotationFile is the on-disk shape of <id>.annotations.json. The identity
// fields are authoritative from the server, not the client.
type AnnotationFile struct {
	ArtifactID   string       `json:"artifactId"`
	ArtifactFile string       `json:"artifactFile"`
	CreatedAt    string       `json:"createdAt"`
	Annotations  []Annotation `json:"annotations"`
}

// Server holds the dependencies shared by the HTTP handlers.
type Server struct {
	store *storage.Store
	index *template.Template
	// lastActivity is the Unix-nanos timestamp of the most recent request,
	// used to drive idle shutdown.
	lastActivity atomic.Int64
}

// New returns a Server backed by the given store, with the index template
// parsed from the embedded assets.
func New(store *storage.Store) (*Server, error) {
	index, err := template.ParseFS(assets.Files, "index.html.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse index template: %w", err)
	}
	s := &Server{store: store, index: index}
	s.lastActivity.Store(time.Now().UnixNano())
	return s, nil
}

// Serve runs the HTTP server on ln until ctx is canceled (e.g. a shutdown
// signal) or, when idle > 0, until no request has arrived for idle. It returns
// nil on a clean shutdown, draining in-flight requests first (up to 5s).
func (s *Server) Serve(ctx context.Context, ln net.Listener, idle time.Duration) error {
	httpSrv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		s.awaitShutdown(ctx, idle)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// awaitShutdown blocks until ctx is done, or (when idle > 0) until the server
// has gone at least idle without a request.
func (s *Server) awaitShutdown(ctx context.Context, idle time.Duration) {
	if idle <= 0 {
		<-ctx.Done()
		return
	}
	tick := idle / 4
	if tick < 50*time.Millisecond {
		tick = 50 * time.Millisecond
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s.idleFor() >= idle {
				return
			}
		}
	}
}

// idleFor reports how long it has been since the last request.
func (s *Server) idleFor() time.Duration {
	return time.Since(time.Unix(0, s.lastActivity.Load()))
}

// trackActivity records the time of each request so idle shutdown can tell when
// the server has gone quiet.
func (s *Server) trackActivity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.lastActivity.Store(time.Now().UnixNano())
		next.ServeHTTP(w, r)
	})
}

// Handler returns the HTTP handler for all routes, wrapped so the server only
// answers loopback requests (defends against DNS rebinding and cross-site
// writes via the user's browser).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleRoot)
	mux.HandleFunc("GET /artifacts", s.handleIndex)
	mux.HandleFunc("GET /view/{id}", s.handleView)
	mux.HandleFunc("GET /_editor/shell.js", s.handleShell)
	mux.HandleFunc("GET /_vendor/mermaid.min.js", s.handleMermaid)
	mux.HandleFunc("POST /annotations/{id}", s.handlePostAnnotations)
	mux.HandleFunc("GET /annotations/{id}", s.handleGetAnnotations)
	return s.trackActivity(localhostOnly(mux))
}

func localhostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) {
			http.Error(w, "forbidden: non-loopback host", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin != "" && !isLoopbackOrigin(origin) {
				http.Error(w, "forbidden: cross-origin request", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackHost(host string) bool {
	h := host
	if hh, _, err := net.SplitHostPort(host); err == nil {
		h = hh
	}
	h = strings.TrimPrefix(strings.TrimSuffix(h, "]"), "[")
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}

func isLoopbackOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return isLoopbackHost(u.Host)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/artifacts", http.StatusFound)
}

type indexRow struct {
	ID   string
	When string
	Size string
}

type indexView struct {
	Count int
	Rows  []indexRow
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	arts, err := s.store.List()
	if err != nil {
		http.Error(w, "failed to list artifacts", http.StatusInternalServerError)
		return
	}
	view := indexView{Count: len(arts)}
	for _, a := range arts {
		view.Rows = append(view.Rows, indexRow{
			ID:   a.ID,
			When: a.ModTime.Format("2006-01-02 15:04"),
			Size: humanSize(a.Size),
		})
	}

	var buf bytes.Buffer
	if err := s.index.Execute(&buf, view); err != nil {
		http.Error(w, "failed to render index", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) handleView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data, err := s.store.ReadArtifact(id)
	if errors.Is(err, storage.ErrInvalidID) || errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "failed to read artifact", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; "+
			"img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
	_, _ = w.Write(injectAssets(data))
}

func (s *Server) handleShell(w http.ResponseWriter, r *http.Request) {
	data, err := assets.Files.ReadFile("shell.js")
	if err != nil {
		http.Error(w, "editor shell unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	_, _ = w.Write(data)
}

// handleMermaid serves the embedded Mermaid runtime so artifacts can load it
// from the local server instead of inlining the 3.4 MB library into every
// generated file.
func (s *Server) handleMermaid(w http.ResponseWriter, r *http.Request) {
	data, err := assets.Files.ReadFile("mermaid.min.js")
	if err != nil {
		http.Error(w, "mermaid runtime unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(data)
}

func (s *Server) handlePostAnnotations(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	exists, err := s.store.ArtifactExists(id)
	if errors.Is(err, storage.ErrInvalidID) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "failed to check artifact", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.NotFound(w, r)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAnnotationBody)
	var af AnnotationFile
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&af); err != nil {
		http.Error(w, "invalid annotation JSON", http.StatusBadRequest)
		return
	}

	// Identity fields are authoritative from the URL, not the client. Stamp
	// timestamps server-side and give each annotation a stable id.
	now := time.Now().UTC().Format(time.RFC3339)
	af.ArtifactID = id
	af.ArtifactFile = id + ".html"
	af.CreatedAt = now
	for i := range af.Annotations {
		if af.Annotations[i].ID == "" {
			af.Annotations[i].ID = fmt.Sprintf("a%d", i+1)
		}
		if af.Annotations[i].CreatedAt == "" {
			af.Annotations[i].CreatedAt = now
		}
	}

	// Encode without HTML escaping so selectors keep their literal '>' etc.
	// in the stored file (they decode identically either way).
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(af); err != nil {
		http.Error(w, "failed to encode annotations", http.StatusInternalServerError)
		return
	}
	data := buf.Bytes()
	if err := s.store.WriteAnnotations(id, data); err != nil {
		http.Error(w, "failed to store annotations", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) handleGetAnnotations(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data, err := s.store.ReadAnnotations(id)
	if errors.Is(err, storage.ErrInvalidID) || errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "failed to read annotations", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(data)
}

// mermaidTag loads the embedded Mermaid runtime and initializes it (strict
// security, theme by prefers-color-scheme). Injected only when the artifact
// contains a .mermaid block. Runtime first, then init, so init runs after the
// runtime has defined window.mermaid.
const mermaidTag = `<script src="/_vendor/mermaid.min.js"></script>
<script>
if (window.mermaid) {
  var dark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
  window.mermaid.initialize({ startOnLoad: true, securityLevel: 'strict', theme: dark ? 'dark' : 'neutral', fontFamily: 'ui-monospace, Menlo, monospace' });
}
</script>`

// injectAssets inserts the editor shell (always) and the Mermaid runtime (only
// when the artifact contains a .mermaid block) just before </body>, leaving
// the artifact's own markup untouched. If there is no </body>, the tags are
// appended.
func injectAssets(html []byte) []byte {
	tags := shellTag
	if bytes.Contains(html, []byte(`class="mermaid"`)) {
		tags = mermaidTag + "\n" + shellTag
	}
	idx := strings.LastIndex(strings.ToLower(string(html)), "</body>")
	if idx == -1 {
		return append(append([]byte{}, html...), []byte("\n"+tags)...)
	}
	out := make([]byte, 0, len(html)+len(tags)+2)
	out = append(out, html[:idx]...)
	out = append(out, []byte(tags+"\n")...)
	out = append(out, html[idx:]...)
	return out
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

// ListenAddr builds the loopback-only bind address. The server must never be
// reachable off-host, so the host is always 127.0.0.1.
func ListenAddr(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}
