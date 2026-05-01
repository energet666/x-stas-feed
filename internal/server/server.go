package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"feed-ai/internal/media"
)

type Server struct {
	mux       *http.ServeMux
	library   *media.Library
	staticDir string
	logger    *log.Logger
}

func New(library *media.Library, staticDir string, logger *log.Logger) *Server {
	s := &Server{
		mux:       http.NewServeMux(),
		library:   library,
		staticDir: staticDir,
		logger:    logger,
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.withLogging(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/feed", s.handleFeed)
	s.mux.HandleFunc("GET /media/{id}", s.handleMedia)
	s.mux.HandleFunc("GET /", s.handleStatic)
}

func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	page, err := s.library.Page(r.URL.Query().Get("cursor"), limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	path, mimeType, err := s.library.PathForID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, path)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	if s.staticDir == "" {
		writeDevFallback(w)
		return
	}

	index := filepath.Join(s.staticDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		writeDevFallback(w)
		return
	}

	requestPath := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if requestPath == "." {
		requestPath = "index.html"
	}
	fullPath := filepath.Join(s.staticDir, requestPath)
	staticRoot, err := filepath.Abs(s.staticDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "static root is invalid")
		return
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil || (absPath != staticRoot && !strings.HasPrefix(absPath, staticRoot+string(os.PathSeparator))) {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(absPath)
	if err == nil && !info.IsDir() {
		http.ServeFile(w, r, absPath)
		return
	}

	http.ServeFile(w, r, index)
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		s.logger.Printf("request method=%s path=%s", r.Method, r.URL.Path)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeDevFallback(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Feed AI</title></head><body><h1>Feed AI API is running</h1><p>Build the frontend with <code>npm --prefix web run build</code> to serve the SPA from Go.</p></body></html>`))
}
