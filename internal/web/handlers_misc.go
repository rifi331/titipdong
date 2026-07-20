package web

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// handleStatic serves vendored assets from web/static.
func (s *Server) handleStatic() http.HandlerFunc {
	fs := http.FileServer(http.Dir("web/static"))
	return func(w http.ResponseWriter, r *http.Request) {
		// chi strips the /static prefix via Route elsewhere; here we serve
		// by trimming manually since we mounted at /static/*.
		r2 := new(http.Request)
		*r2 = *r
		r2.URL.Path = strings.TrimPrefix(r.URL.Path, "/static")
		if r2.URL.Path == "" || r2.URL.Path == "/" {
			r2.URL.Path = "/"
		}
		fs.ServeHTTP(w, r2)
	}
}

// handleUpload serves a saved photo from the uploads directory.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	// Reject path traversal.
	clean := filepath.Base(name)
	if clean != name || clean == "" || strings.Contains(clean, "/") {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.cfg.UploadsDir, clean))
}

// handleManifest serves the PWA manifest (templated with BaseURL).
func (s *Server) handleManifest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		http.ServeFile(w, r, filepath.Join("web/static", "manifest.json"))
	}
}

// handleServiceWorker serves the service worker JS.
func (s *Server) handleServiceWorker() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, filepath.Join("web/static", "sw.js"))
	}
}
