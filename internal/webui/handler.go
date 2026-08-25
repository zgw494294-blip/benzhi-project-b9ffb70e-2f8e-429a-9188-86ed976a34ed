package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
)

//go:embed web/*
var assets embed.FS

func Handler() http.Handler {
	root, _ := fs.Sub(assets, "web")
	index, _ := fs.ReadFile(root, "index.html")
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			if r.Method == "GET" {
				_, _ = w.Write(index)
			}
			return
		}
		if path.Ext(r.URL.Path) == "" {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}
