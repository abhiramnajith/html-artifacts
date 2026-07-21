// Package assets holds the server's embedded static files: the annotation
// editor shell and the artifact index template. They are compiled into the
// binary via go:embed so target machines need no runtime files.
package assets

import "embed"

//go:embed shell.js index.html.tmpl mermaid.min.js
var Files embed.FS
