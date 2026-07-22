package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abhiramnajith/vellum/server/internal/storage"
)

// newTestServer builds a Server over a temp artifacts dir seeded with one
// artifact, and returns the handler plus the seeded id.
func newTestServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	dir := t.TempDir()
	id := "react-vs-vue-20260721-103000"
	body := "<!doctype html><html><body><h1>React vs Vue</h1></body></html>"
	if err := os.WriteFile(filepath.Join(dir, id+".html"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	srv, err := New(storage.New(dir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv.Handler(), id
}

func do(t *testing.T, h http.Handler, method, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Host = "127.0.0.1:47600"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func TestRootRedirectsToArtifacts(t *testing.T) {
	h, _ := newTestServer(t)
	resp := do(t, h, "GET", "/")
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("GET /: want redirect, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/artifacts" {
		t.Fatalf("GET /: want Location /artifacts, got %q", loc)
	}
}

func TestArtifactsIndexListsArtifacts(t *testing.T) {
	h, id := newTestServer(t)
	resp := do(t, h, "GET", "/artifacts")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /artifacts: want 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, id) {
		t.Fatalf("GET /artifacts: body does not list id %q", id)
	}
	if !strings.Contains(body, "/view/"+id) {
		t.Fatalf("GET /artifacts: body has no link to /view/%s", id)
	}
}

func TestViewServesArtifactWithInjectedShell(t *testing.T) {
	h, id := newTestServer(t)
	resp := do(t, h, "GET", "/view/"+id)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /view/%s: want 200, got %d", id, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("GET /view: want text/html, got %q", ct)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "React vs Vue") {
		t.Fatal("GET /view: artifact content missing")
	}
	if !strings.Contains(body, "/_editor/shell.js") {
		t.Fatal("GET /view: editor shell was not injected")
	}
}

func TestViewUnknownIDReturns404(t *testing.T) {
	h, _ := newTestServer(t)
	resp := do(t, h, "GET", "/view/nope-20260101-000000")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /view/<missing>: want 404, got %d", resp.StatusCode)
	}
}

func TestViewTraversalIsRefused(t *testing.T) {
	h, _ := newTestServer(t)
	// Percent-encoded and literal traversal attempts must never 200 or leak a
	// file outside the artifacts dir.
	for _, bad := range []string{
		"/view/..%2f..%2fetc%2fpasswd",
		"/view/..",
		"/view/A",   // uppercase — invalid id
		"/view/a_b", // underscore — invalid id
	} {
		resp := do(t, h, "GET", bad)
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("GET %s: traversal/invalid id returned 200", bad)
		}
	}
}

func TestEditorShellIsServed(t *testing.T) {
	h, _ := newTestServer(t)
	resp := do(t, h, "GET", "/_editor/shell.js")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /_editor/shell.js: want 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("GET /_editor/shell.js: want javascript content-type, got %q", ct)
	}
}

func TestMermaidRuntimeIsServed(t *testing.T) {
	h, _ := newTestServer(t)
	resp := do(t, h, "GET", "/_vendor/mermaid.min.js")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /_vendor/mermaid.min.js: want 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("GET /_vendor/mermaid.min.js: want javascript content-type, got %q", ct)
	}
}

func TestListenAddrIsLoopbackOnly(t *testing.T) {
	// The server must never be reachable off-host: the bind address is always
	// 127.0.0.1, never a wildcard like ":7777" or "0.0.0.0".
	got := ListenAddr(7777)
	if got != "127.0.0.1:7777" {
		t.Fatalf("ListenAddr(7777) = %q, want 127.0.0.1:7777", got)
	}
	if strings.HasPrefix(got, ":") || strings.HasPrefix(got, "0.0.0.0") {
		t.Fatalf("ListenAddr must not bind a wildcard interface, got %q", got)
	}
}

func doPost(t *testing.T, h http.Handler, path, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Host = "127.0.0.1:47600"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func TestAnnotationsRoundTripAndServerStampsID(t *testing.T) {
	h, id := newTestServer(t)

	// The client claims a DIFFERENT artifactId; the server must overwrite it
	// with the id from the URL.
	body := `{"artifactId":"someone-elses-artifact","annotations":[` +
		`{"selector":"#comparison-table","selectedText":"","comment":"add a bundle-size row"}]}`

	post := doPost(t, h, "/annotations/"+id, body)
	if post.StatusCode != http.StatusOK {
		t.Fatalf("POST /annotations/%s: want 200, got %d", id, post.StatusCode)
	}

	get := do(t, h, "GET", "/annotations/"+id)
	if get.StatusCode != http.StatusOK {
		t.Fatalf("GET /annotations/%s: want 200, got %d", id, get.StatusCode)
	}
	stored := readBody(t, get)
	if strings.Contains(stored, "someone-elses-artifact") {
		t.Fatal("server did not overwrite the client-supplied artifactId")
	}
	if !strings.Contains(stored, id) {
		t.Fatalf("stored annotations do not carry the URL id %q: %s", id, stored)
	}
	if !strings.Contains(stored, "add a bundle-size row") {
		t.Fatalf("stored annotations missing the comment: %s", stored)
	}
}

func TestPostAnnotationsForMissingArtifactIs404(t *testing.T) {
	h, _ := newTestServer(t)
	resp := doPost(t, h, "/annotations/nope-20260101-000000", `{"annotations":[]}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("POST for missing artifact: want 404, got %d", resp.StatusCode)
	}
}

func TestPostAnnotationsInvalidJSONIs400(t *testing.T) {
	h, id := newTestServer(t)
	resp := doPost(t, h, "/annotations/"+id, `{not json`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST invalid JSON: want 400, got %d", resp.StatusCode)
	}
}

func TestGetAnnotationsWhenNoneIs404(t *testing.T) {
	h, id := newTestServer(t)
	resp := do(t, h, "GET", "/annotations/"+id)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET annotations (none written): want 404, got %d", resp.StatusCode)
	}
}

func TestAnnotationsTraversalRefused(t *testing.T) {
	h, _ := newTestServer(t)
	if resp := doPost(t, h, "/annotations/..%2f..%2fetc%2fpasswd", `{}`); resp.StatusCode == http.StatusOK {
		t.Fatal("POST annotations traversal returned 200")
	}
	if resp := do(t, h, "GET", "/annotations/A"); resp.StatusCode == http.StatusOK {
		t.Fatal("GET annotations invalid id returned 200")
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func getViewBody(t *testing.T, h http.Handler, id string) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/view/"+id, nil)
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return readBody(t, rec.Result())
}

func TestViewInjectsMermaidOnlyWithDiagram(t *testing.T) {
	dir := t.TempDir()
	withDiagram := `<!doctype html><html><body><div class="mermaid">graph TD;A-->B;</div></body></html>`
	plain := `<!doctype html><html><body><h1>no diagram</h1></body></html>`
	writeFile(t, dir, "with-20260101-000000.html", withDiagram)
	writeFile(t, dir, "plain-20260101-000000.html", plain)
	srv, _ := New(storage.New(dir))
	h := srv.Handler()

	withBody := getViewBody(t, h, "with-20260101-000000")
	if !strings.Contains(withBody, "/_vendor/mermaid.min.js") {
		t.Fatal("diagram artifact missing injected Mermaid runtime")
	}
	if !strings.Contains(withBody, "mermaid.initialize") || !strings.Contains(withBody, "securityLevel") {
		t.Fatal("diagram artifact missing Mermaid init (strict/theme)")
	}
	plainBody := getViewBody(t, h, "plain-20260101-000000")
	if strings.Contains(plainBody, "/_vendor/mermaid.min.js") {
		t.Fatal("plain artifact should not get Mermaid runtime")
	}
	if !strings.Contains(plainBody, "/_editor/shell.js") {
		t.Fatal("editor shell should always be injected")
	}
}

func TestViewSetsRestrictiveCSP(t *testing.T) {
	h, id := newTestServer(t)
	req := httptest.NewRequest("GET", "/view/"+id, nil)
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	csp := rec.Result().Header.Get("Content-Security-Policy")
	// connect-src blocks fetch/XHR exfil; form-action blocks form-POST exfil;
	// frame-ancestors blocks clickjacking of the local viewer.
	for _, want := range []string{"connect-src 'self'", "form-action 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, want) {
			t.Fatalf("view CSP missing %q: %q", want, csp)
		}
	}
}
