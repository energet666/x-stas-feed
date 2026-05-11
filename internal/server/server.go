package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"feed-ai/internal/media"
)

const (
	mediaCacheControl = "public, max-age=3600"
	uploadMaxBytes    = 1 << 30
)

type Server struct {
	mux       *http.ServeMux
	library   *media.Library
	comments  *commentHub
	ships     *shipHub
	boards    *media.BoardStore
	boardHub  *boardHub
	staticDir string
	logger    *log.Logger
}

func New(library *media.Library, contentRoot string, staticDir string, logger *log.Logger) *Server {
	boardStore := media.NewBoardStore(contentRoot)
	if err := boardStore.Init(); err != nil {
		logger.Printf("board store init failed error=%v", err)
	}
	s := &Server{
		mux:       http.NewServeMux(),
		library:   library,
		comments:  newCommentHub(),
		ships:     newShipHub(),
		boards:    boardStore,
		boardHub:  newBoardHub(),
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
	s.mux.HandleFunc("POST /api/feed/favorites", s.handleFavoriteFeed)
	s.mux.HandleFunc("GET /api/activity", s.handleActivity)
	s.mux.HandleFunc("POST /api/uploads", s.handleUploads)
	s.mux.HandleFunc("GET /api/comments/events", s.handleCommentEvents)
	s.mux.HandleFunc("GET /api/ships/socket", s.handleShipSocket)
	s.mux.HandleFunc("GET /api/media/{id}", s.handleMediaItem)
	s.mux.HandleFunc("GET /api/media/{id}/comments", s.handleComments)
	s.mux.HandleFunc("POST /api/media/{id}/comments", s.handleCreateComment)
	s.mux.HandleFunc("POST /api/media/{id}/comments/{commentID}/likes", s.handleCreateCommentLike)
	s.mux.HandleFunc("POST /api/media/{id}/likes", s.handleCreateLike)
	s.mux.HandleFunc("GET /api/media/{id}/cover", s.handleMediaCover)
	s.mux.HandleFunc("GET /api/media/{id}/poster", s.handleMediaPoster)
	s.mux.HandleFunc("GET /api/boards", s.handleListBoards)
	s.mux.HandleFunc("POST /api/boards", s.handleCreateBoard)
	s.mux.HandleFunc("GET /api/boards/events", s.handleAllBoardEvents)
	s.mux.HandleFunc("GET /api/boards/{id}", s.handleGetBoard)
	s.mux.HandleFunc("POST /api/boards/{id}/strokes", s.handleCreateStroke)
	s.mux.HandleFunc("GET /api/boards/{id}/events", s.handleBoardEvents)
	s.mux.HandleFunc("GET /media/{id}", s.handleMedia)
	s.mux.HandleFunc("GET /", s.handleStatic)
}

type uploadError struct {
	Filename string `json:"filename"`
	Error    string `json:"error"`
}

type uploadResponse struct {
	Items  []media.Item  `json:"items"`
	Errors []uploadError `json:"errors,omitempty"`
}

type favoriteFeedRequest struct {
	IDs    []string `json:"ids"`
	Cursor string   `json:"cursor"`
	Limit  int      `json:"limit"`
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

func (s *Server) handleFavoriteFeed(w http.ResponseWriter, r *http.Request) {
	var request favoriteFeedRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid favorite feed payload")
		return
	}

	page, err := s.library.FavoritePage(request.IDs, request.Cursor, request.Limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.library.Activity(limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string][]media.ActivityItem{"items": items})
}

func (s *Server) handleMediaItem(w http.ResponseWriter, r *http.Request) {
	item, err := s.library.ItemForID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUploads(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		writeError(w, http.StatusBadRequest, "upload request must be multipart/form-data")
		return
	}
	if r.ContentLength > uploadMaxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "upload request is too large")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, uploadMaxBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid upload payload")
		return
	}

	var response uploadResponse
	seenFilePart := false
	var pendingModifiedAt []time.Time

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if isRequestTooLarge(err) {
				writeError(w, http.StatusRequestEntityTooLarge, "upload request is too large")
				return
			}
			writeError(w, http.StatusBadRequest, "invalid upload payload")
			return
		}

		if part.FormName() == "modifiedAt" {
			modifiedAt, ok := readUploadModifiedAt(part)
			_ = part.Close()
			if ok {
				pendingModifiedAt = append(pendingModifiedAt, modifiedAt)
			}
			continue
		}

		if part.FormName() != "files" {
			_ = part.Close()
			continue
		}

		seenFilePart = true
		filename := partFilename(part)
		var sourceModifiedAt time.Time
		if len(pendingModifiedAt) > 0 {
			sourceModifiedAt = pendingModifiedAt[0]
			pendingModifiedAt = pendingModifiedAt[1:]
		}
		item, err := s.library.SaveUploadWithModifiedAt(filename, part, sourceModifiedAt)
		_ = part.Close()
		if err != nil {
			if isRequestTooLarge(err) {
				writeError(w, http.StatusRequestEntityTooLarge, "upload request is too large")
				return
			}
			response.Errors = append(response.Errors, uploadError{Filename: filename, Error: err.Error()})
			continue
		}
		response.Items = append(response.Items, item)
	}

	if len(response.Items) == 0 {
		if !seenFilePart && len(response.Errors) == 0 {
			writeError(w, http.StatusBadRequest, "no files were uploaded")
			return
		}
		writeJSON(w, http.StatusBadRequest, response)
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

func partFilename(part *multipart.Part) string {
	_, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if err != nil {
		return part.FileName()
	}
	return params["filename"]
}

func readUploadModifiedAt(part *multipart.Part) (time.Time, bool) {
	bytes, err := io.ReadAll(io.LimitReader(part, 64))
	if err != nil {
		return time.Time{}, false
	}
	millis, err := strconv.ParseInt(strings.TrimSpace(string(bytes)), 10, 64)
	if err != nil || millis <= 0 {
		return time.Time{}, false
	}
	return time.UnixMilli(millis).UTC(), true
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

func (s *Server) handleCreateLike(w http.ResponseWriter, r *http.Request) {
	likeCount, err := s.library.AddLike(r.PathValue("id"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.comments.publishLike(r.PathValue("id"), likeCount)
	writeJSON(w, http.StatusCreated, map[string]int{"likeCount": likeCount})
}

func (s *Server) handleCreateCommentLike(w http.ResponseWriter, r *http.Request) {
	comment, err := s.library.AddCommentLike(r.PathValue("id"), r.PathValue("commentID"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, media.ErrCommentNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.comments.publishCommentLike(r.PathValue("id"), comment.ID, comment.LikeCount)
	writeJSON(w, http.StatusCreated, map[string]int{"likeCount": comment.LikeCount})
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
			data, err := json.Marshal(event.Data)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Name, data)
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

func (s *Server) handleMediaCover(w http.ResponseWriter, r *http.Request) {
	path, err := s.library.CoverForID(r.PathValue("id"))
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

func (s *Server) handleCreateBoard(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid board payload")
		return
	}

	info, err := s.boards.Create(request.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create board")
		return
	}

	s.logger.Printf("board created id=%s name=%s", info.ID, info.Name)
	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) handleListBoards(w http.ResponseWriter, r *http.Request) {
	boards := s.boards.ListBoards()
	writeJSON(w, http.StatusOK, map[string][]media.BoardInfo{"boards": boards})
}

func (s *Server) handleGetBoard(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	info, err := s.boards.Get(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	strokes, err := s.boards.Strokes(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"board":   info,
		"strokes": strokes,
	})
}

func (s *Server) handleCreateStroke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var request struct {
		Tool   string      `json:"tool"`
		Points [][]float64 `json:"points"`
		Color  string      `json:"color"`
		Size   float64     `json:"size"`
		Author string      `json:"author"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid stroke payload")
		return
	}

	stroke, err := s.boards.AddStroke(id, request.Tool, request.Points, request.Color, request.Size, request.Author)
	if err != nil {
		if errors.Is(err, media.ErrBoardNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.boardHub.publishStroke(id, stroke)
	writeJSON(w, http.StatusCreated, stroke)
}

func (s *Server) handleAllBoardEvents(w http.ResponseWriter, r *http.Request) {
	ch := s.boardHub.subscribeAll()
	defer s.boardHub.unsubscribeAll(ch)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {
		select {
		case event := <-ch:
			data, err := json.Marshal(event.Data)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Name, data)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleBoardEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, err := s.boards.Get(id); err != nil {
		http.NotFound(w, r)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	events := s.boardHub.subscribe(id)
	defer s.boardHub.unsubscribe(id, events)

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-events:
			data, err := json.Marshal(event.Data)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Name, data)
			flusher.Flush()
		}
	}
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

func isRequestTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}

func writeDevFallback(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Feed AI</title></head><body><h1>Feed AI API is running</h1><p>Build the frontend with <code>npm --prefix web run build</code> to serve the SPA from Go.</p></body></html>`))
}
