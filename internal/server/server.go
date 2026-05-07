package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"feed-ai/internal/media"
)

const mediaCacheControl = "public, max-age=3600"

type Server struct {
	mux       *http.ServeMux
	library   *media.Library
	comments  *commentHub
	ships     *shipHub
	staticDir string
	logger    *log.Logger
}

func New(library *media.Library, staticDir string, logger *log.Logger) *Server {
	s := &Server{
		mux:       http.NewServeMux(),
		library:   library,
		comments:  newCommentHub(),
		ships:     newShipHub(),
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
	s.mux.HandleFunc("GET /api/comments/events", s.handleCommentEvents)
	s.mux.HandleFunc("GET /api/ships/socket", s.handleShipSocket)
	s.mux.HandleFunc("GET /api/media/{id}/comments", s.handleComments)
	s.mux.HandleFunc("POST /api/media/{id}/comments", s.handleCreateComment)
	s.mux.HandleFunc("GET /api/media/{id}/poster", s.handleMediaPoster)
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

func (s *Server) handleComments(w http.ResponseWriter, r *http.Request) {
	comments, err := s.library.CommentsForID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]media.Comment{"comments": comments})
}

func (s *Server) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Text   string `json:"text"`
		Author string `json:"author"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid comment payload")
		return
	}

	comment, err := s.library.AddComment(r.PathValue("id"), request.Text, request.Author)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Location", r.URL.Path+"/"+comment.ID)
	s.comments.publish(r.PathValue("id"), comment)
	writeJSON(w, http.StatusCreated, comment)
}

func (s *Server) handleCommentEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	events := s.comments.subscribe()
	defer s.comments.unsubscribe(events)

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-events:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: comment\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	path, mimeType, err := s.library.PathForID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", mediaCacheControl)
	http.ServeFile(w, r, path)
}

func (s *Server) handleMediaPoster(w http.ResponseWriter, r *http.Request) {
	seconds, err := strconv.ParseFloat(r.URL.Query().Get("time"), 64)
	if err != nil && r.URL.Query().Get("time") != "" {
		writeError(w, http.StatusBadRequest, "invalid poster time")
		return
	}

	path, err := s.library.PosterForID(r.PathValue("id"), seconds)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
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
