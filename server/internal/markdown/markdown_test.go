package markdown

import (
	"strings"
	"testing"
)

func TestRenderBasics(t *testing.T) {
	md := "# Title\n\nSome **bold** and `code`.\n\n- [ ] todo\n- [x] done\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\n```go\nfmt.Println(\"hi\")\n```\n"
	got := Render(md)
	for _, want := range []string{
		"<h1>Title</h1>",
		"<strong>bold</strong>",
		"<code>code</code>",
		"<li>", "todo", "done",
		"<div class=\"table-scroll\"><table>",
		"<pre><code>", "fmt.Println",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Render output missing %q\n---\n%s", want, got)
		}
	}
	if strings.Contains(got, "<script") {
		t.Fatal("Render must not emit raw <script>")
	}
}

func TestRenderEscapesHTML(t *testing.T) {
	got := Render("a <img src=x onerror=alert(1)> b")
	if strings.Contains(got, "<img") {
		t.Fatalf("HTML in markdown text must be escaped, got: %s", got)
	}
}

func TestRenderDropsUnsafeLinkSchemes(t *testing.T) {
	for _, in := range []string{
		"[click](javascript:alert(1))",
		"[x](JavaScript:alert(1))",
		"[y](data:text/html,<script>alert(1)</script>)",
		"[z](vbscript:msgbox(1))",
	} {
		got := Render(in)
		if strings.Contains(got, "href=\"javascript:") || strings.Contains(strings.ToLower(got), "href=\"javascript:") ||
			strings.Contains(got, "href=\"data:") || strings.Contains(strings.ToLower(got), "href=\"vbscript:") {
			t.Fatalf("unsafe scheme survived in href for %q: %s", in, got)
		}
	}
}

func TestRenderKeepsSafeLinks(t *testing.T) {
	for _, in := range []string{"[a](https://example.com)", "[b](http://x.io/p)", "[c](/rel/path)", "[d](#anchor)", "[e](mailto:x@y.z)"} {
		got := Render(in)
		if !strings.Contains(got, "<a href=") {
			t.Fatalf("safe link was dropped for %q: %s", in, got)
		}
	}
}

func TestRenderCodeSpanNotReprocessed(t *testing.T) {
	got := Render("`**not bold**`")
	if strings.Contains(got, "<strong>") {
		t.Fatalf("bold was applied inside a code span: %s", got)
	}
	got2 := Render("`[x](javascript:alert(1))`")
	if strings.Contains(got2, "<a ") {
		t.Fatalf("a link was created inside a code span: %s", got2)
	}
}
