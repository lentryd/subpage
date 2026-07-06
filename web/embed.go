// Package web provides the embedded frontend build files.
//
// Run `bun run build` inside web/ before `go build` — it produces the
// dist/ directory embedded below. Until dist/ exists (with at least a
// .gitkeep), `go build` will fail on the //go:embed directive.
package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

//go:generate bun install
//go:generate bun run build

//go:embed all:dist
var distFS embed.FS

// StaticFS is the dist/ directory rooted at "/", ready to hand to
// http.FileServer.
var StaticFS = func() http.FileSystem {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}()

// IndexTemplate is the built dist/index.html, still carrying the
// {{.MetaTitle}}/{{.MetaDescription}}/{{.PanelData}} placeholders left
// untouched by the Bun bundler (it only rewrites <link>/<script> tags).
// Rendered server-side once per browser request to a shortUuid.
var IndexTemplate = template.Must(template.ParseFS(distFS, "dist/index.html"))
