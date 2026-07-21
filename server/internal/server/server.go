// Package server wires the HTTP handlers (view, list, editor shell) onto a
// storage.Store. It never inspects or branches on which agent produced or
// consumes an artifact — the contract is files and HTTP only.
package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	assets "github.com/abhiramnajith/html-artifacts/server/embed"
	"github.com/abhiramnajith/html-artifacts/server/internal/storage"
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
}

// New returns a Server backed by the given store, with the index template
// parsed from the embedded assets.
func New(store *storage.Store) (*Server, error) {
	index, err := template.ParseFS(assets.Files, "index.html.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse index template: %w", err)
	}
	return &Server{store: store, index: index}, nil
}

// Handler returns the HTTP handler for all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleRoot)
	mux.HandleFunc("GET /artifacts", s.handleIndex)
	mux.HandleFunc("GET /view/{id}", s.handleView)
	mux.HandleFunc("GET /_editor/shell.js", s.handleShell)
	mux.HandleFunc("POST /annotations/{id}", s.handlePostAnnotations)
	mux.HandleFunc("GET /annotations/{id}", s.handleGetAnnotations)
	return mux
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
	_, _ = w.Write(injectShell(data))
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

// injectShell inserts the editor <script> just before the closing </body> tag,
// leaving the artifact's own markup untouched. If there is no </body>, the tag
// is appended.
func injectShell(html []byte) []byte {
	idx := strings.LastIndex(strings.ToLower(string(html)), "</body>")
	if idx == -1 {
		return append(append([]byte{}, html...), []byte("\n"+shellTag)...)
	}
	out := make([]byte, 0, len(html)+len(shellTag)+2)
	out = append(out, html[:idx]...)
	out = append(out, []byte(shellTag+"\n")...)
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
