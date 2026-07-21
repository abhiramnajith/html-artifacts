// Package markdown is a tiny, dependency-free Markdown-to-HTML renderer for the
// constructs used in project docs: headings, fenced code, inline code, GFM
// tables, ordered/unordered lists (incl. task checkboxes), blockquotes,
// horizontal rules, bold, and links. Output is plain semantic HTML styled by
// base.html's element-level CSS. All text is HTML-escaped.
package markdown

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

var (
	reHeading  = regexp.MustCompile(`^(#{1,4})\s+(.*)$`)
	reHR       = regexp.MustCompile(`^---+\s*$`)
	reList     = regexp.MustCompile(`^\s*([-*]|\d+\.)\s+`)
	reOrdered  = regexp.MustCompile(`^\s*\d+\.\s+`)
	reTask     = regexp.MustCompile(`^\[([ xX])\]\s+(.*)$`)
	reInlCode  = regexp.MustCompile("`([^`]+)`")
	reBold     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reLink     = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reTableSep = regexp.MustCompile(`^\s*\|?[\s:|-]+\|?\s*$`)
)

// safeURL reports whether url is safe to place in an href attribute.
// The URL has already been through html.EscapeString by the time this is
// called, so any embedded quotes/angle-brackets are already neutralized;
// this only needs to check the scheme.
func safeURL(url string) bool {
	if url == "" {
		return false
	}
	if strings.HasPrefix(url, "#") {
		return true
	}
	lower := strings.ToLower(url)
	for _, scheme := range []string{"http://", "https://", "mailto:"} {
		if strings.HasPrefix(lower, scheme) {
			return true
		}
	}
	// Relative URLs: either rooted at "/" or containing no scheme separator
	// at all. Anything with a ":" before the first "/" is treated as a
	// scheme (javascript:, data:, vbscript:, etc.) and rejected.
	if strings.HasPrefix(url, "/") {
		return true
	}
	if !strings.Contains(url, ":") {
		return true
	}
	return false
}

// applyBoldAndLinks runs the bold and link substitutions over a string that
// contains no inline-code spans (callers must exempt code-span content).
func applyBoldAndLinks(s string) string {
	s = reBold.ReplaceAllString(s, "<strong>$1</strong>")
	s = reLink.ReplaceAllStringFunc(s, func(m string) string {
		sub := reLink.FindStringSubmatch(m)
		text, url := sub[1], sub[2]
		if !safeURL(url) {
			return text
		}
		return `<a href="` + url + `">` + text + `</a>`
	})
	return s
}

// inlineFmt escapes HTML in s, then applies inline markdown formatting
// (code spans, bold, links). Inline code spans are carved out first and
// their contents are exempt from the bold/link passes, so that formatting
// syntax or dangerous link schemes written inside a code span are rendered
// literally rather than being turned into live markup.
func inlineFmt(s string) string {
	s = html.EscapeString(s)

	var b strings.Builder
	last := 0
	for _, idx := range reInlCode.FindAllStringSubmatchIndex(s, -1) {
		// idx: [fullStart, fullEnd, groupStart, groupEnd]
		b.WriteString(applyBoldAndLinks(s[last:idx[0]]))
		b.WriteString("<code>")
		b.WriteString(s[idx[2]:idx[3]])
		b.WriteString("</code>")
		last = idx[1]
	}
	b.WriteString(applyBoldAndLinks(s[last:]))
	return b.String()
}

func cells(row string) []string {
	row = strings.Trim(strings.TrimSpace(row), "|")
	parts := strings.Split(row, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// Render converts Markdown to an HTML fragment.
func Render(md string) string {
	lines := strings.Split(md, "\n")
	var out, para []string
	flush := func() {
		if len(para) > 0 {
			out = append(out, "<p>"+inlineFmt(strings.Join(para, " "))+"</p>")
			para = nil
		}
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "```"):
			flush()
			i++
			var code []string
			for i < len(lines) && !strings.HasPrefix(lines[i], "```") {
				code = append(code, lines[i])
				i++
			}
			out = append(out, "<pre><code>"+html.EscapeString(strings.Join(code, "\n"))+"</code></pre>")
		case reHeading.MatchString(line):
			flush()
			m := reHeading.FindStringSubmatch(line)
			lvl := len(m[1])
			out = append(out, fmt.Sprintf("<h%d>%s</h%d>", lvl, inlineFmt(m[2]), lvl))
		case reHR.MatchString(line):
			flush()
			out = append(out, "<hr>")
		case strings.HasPrefix(strings.TrimSpace(line), "|") && i+1 < len(lines) && reTableSep.MatchString(lines[i+1]):
			flush()
			header := cells(line)
			i += 2
			var body [][]string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|") {
				body = append(body, cells(lines[i]))
				i++
			}
			i--
			var b strings.Builder
			b.WriteString(`<div class="table-scroll"><table><thead><tr>`)
			for _, c := range header {
				b.WriteString("<th>" + inlineFmt(c) + "</th>")
			}
			b.WriteString("</tr></thead><tbody>")
			for _, row := range body {
				b.WriteString("<tr>")
				for _, c := range row {
					b.WriteString("<td>" + inlineFmt(c) + "</td>")
				}
				b.WriteString("</tr>")
			}
			b.WriteString("</tbody></table></div>")
			out = append(out, b.String())
		case strings.HasPrefix(line, ">"):
			flush()
			var q []string
			for i < len(lines) && strings.HasPrefix(lines[i], ">") {
				q = append(q, strings.TrimSpace(strings.TrimPrefix(lines[i], ">")))
				i++
			}
			i--
			out = append(out, "<blockquote>"+inlineFmt(strings.Join(q, " "))+"</blockquote>")
		case reList.MatchString(line):
			flush()
			ordered := reOrdered.MatchString(line)
			var items []string
			for i < len(lines) && reList.MatchString(lines[i]) {
				item := reList.ReplaceAllString(lines[i], "")
				if cb := reTask.FindStringSubmatch(item); cb != nil {
					mark := "☐"
					if strings.EqualFold(cb[1], "x") {
						mark = "☑"
					}
					items = append(items, `<li><span class="badge badge--muted">`+mark+"</span> "+inlineFmt(cb[2])+"</li>")
				} else {
					items = append(items, "<li>"+inlineFmt(item)+"</li>")
				}
				i++
			}
			i--
			tag := "ul"
			if ordered {
				tag = "ol"
			}
			out = append(out, "<"+tag+">"+strings.Join(items, "")+"</"+tag+">")
		case strings.TrimSpace(line) == "":
			flush()
		default:
			para = append(para, strings.TrimSpace(line))
		}
	}
	flush()
	return strings.Join(out, "\n")
}
