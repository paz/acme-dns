package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

// Embedded filesystem for templates and static files
//
//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/css/* static/js/* static/img/*
var staticFS embed.FS

// GetTemplates loads templates from embedded filesystem
func GetTemplates() (*template.Template, error) {
	return template.ParseFS(templatesFS, "templates/*.html")
}

// GetStaticHandler returns an http.Handler for serving static files
func GetStaticHandler() http.Handler {
	// Strip the "static" prefix from the embedded filesystem
	fsys, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(fsys))
}
