package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
)

//go:embed web/*
var webFiles embed.FS

func (a *API) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func (a *API) Asset(w http.ResponseWriter, r *http.Request) {
	name := path.Base(r.PathValue("name"))
	contentType := map[string]string{"app.js": "text/javascript; charset=utf-8", "print.js": "text/javascript; charset=utf-8", "style.css": "text/css; charset=utf-8", "workbench.css": "text/css; charset=utf-8"}[name]
	if contentType == "" {
		http.NotFound(w, r)
		return
	}
	sub, err := fs.Sub(webFiles, "web")
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	http.ServeFileFS(w, r, sub, name)
}
