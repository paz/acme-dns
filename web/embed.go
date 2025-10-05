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
	// Create a new template set
	tmpl := template.New("")

	// First parse the layout/base template
	tmpl, err := tmpl.ParseFS(templatesFS, "templates/layout.html")
	if err != nil {
		return nil, err
	}

	// Then parse all other templates which define content blocks
	tmpl, err = tmpl.ParseFS(templatesFS, "templates/login.html", "templates/dashboard.html",
		"templates/profile.html", "templates/register.html", "templates/admin.html")
	if err != nil {
		return nil, err
	}

	return tmpl, nil
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
