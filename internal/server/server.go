package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"feed-ai/internal/game"
	"feed-ai/internal/media"
)

const (
	mediaCacheControl  = "public, max-age=3600"
	uploadMaxBytes     = 5 * 1024 * 1024 * 1024
	uploadMaxSizeLabel = "5 GiB"
	boardImageMaxBytes = 25 * 1024 * 1024
)

type Server struct {
	mux       *http.ServeMux
	library   *media.Library
	comments  *commentHub
	ships     *game.World
	scores    *media.GameScoreStore
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
	library.UseBoardStore(boardStore)
	s := &Server{
		mux:       http.NewServeMux(),
		library:   library,
		comments:  newCommentHub(),
		scores:    media.NewGameScoreStore(contentRoot),
		boards:    boardStore,
		boardHub:  newBoardHub(),
		staticDir: staticDir,
		logger:    logger,
	}
	s.ships = game.NewWorld(func(name string, score int) error {
		_, err := s.scores.Add(name, score)
		return err
	})
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
	s.mux.HandleFunc("GET /api/ships/scores", s.handleShipScores)
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
	s.mux.HandleFunc("GET /api/boards/{id}/background", s.handleBoardBackground)
	s.mux.HandleFunc("GET /api/boards/{id}/assets/{assetID}", s.handleBoardAsset)
	s.mux.HandleFunc("GET /api/boards/{id}", s.handleGetBoard)
	s.mux.HandleFunc("POST /api/boards/{id}/strokes", s.handleCreateStroke)
	s.mux.HandleFunc("POST /api/boards/{id}/images", s.handleCreateBoardImage)
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
	index, err := strconv.Atoi(r.URL.Query().Get("index"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feed index")
		return
	}

	item, err := s.library.IndexedItem(index)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "feed item not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
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
		writeUploadTooLarge(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, uploadMaxBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid upload payload")
		return
	}

	var response uploadResponse
	var uploadFilename string
	var uploadModifiedAt time.Time
	var uploadTemp *os.File
	var pendingModifiedAt []time.Time

	defer func() {
		if uploadTemp != nil {
			name := uploadTemp.Name()
			_ = uploadTemp.Close()
			_ = os.Remove(name)
		}
	}()

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if isRequestTooLarge(err) {
				writeUploadTooLarge(w)
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

		if uploadTemp != nil {
			_ = part.Close()
			writeError(w, http.StatusBadRequest, "only one file can be uploaded at a time")
			return
		}

		filename := partFilename(part)
		var sourceModifiedAt time.Time
		if len(pendingModifiedAt) > 0 {
			sourceModifiedAt = pendingModifiedAt[0]
			pendingModifiedAt = pendingModifiedAt[1:]
		}

		temp, err := os.CreateTemp("", "feed-ai-upload-*")
		if err != nil {
			_ = part.Close()
			writeError(w, http.StatusInternalServerError, "upload could not be stored")
			return
		}
		_, copyErr := io.Copy(temp, part)
		_ = part.Close()
		if copyErr != nil {
			_ = temp.Close()
			_ = os.Remove(temp.Name())
			if isRequestTooLarge(copyErr) {
				writeUploadTooLarge(w)
				return
			}
			writeError(w, http.StatusBadRequest, "invalid upload payload")
			return
		}
		uploadFilename = filename
		uploadModifiedAt = sourceModifiedAt
		uploadTemp = temp
	}

	if uploadTemp == nil {
		writeError(w, http.StatusBadRequest, "no files were uploaded")
		return
	}

	if _, err := uploadTemp.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusInternalServerError, "upload could not be read")
		return
	}

	item, uploadErr := s.library.SaveUploadWithModifiedAt(uploadFilename, uploadTemp, uploadModifiedAt)
	if uploadErr != nil {
		if isRequestTooLarge(uploadErr) {
			writeUploadTooLarge(w)
			return
		}
		response.Errors = append(response.Errors, uploadError{Filename: uploadFilename, Error: uploadErr.Error()})
		writeJSON(w, http.StatusBadRequest, response)
		return
	}

	response.Items = append(response.Items, item)
	s.publishFeedItemCreated(item.ID)
	writeJSON(w, http.StatusCreated, response)
}

func (s *Server) publishFeedItemCreated(id string) {
	item, err := s.library.IndexedItemForID(id)
	if err != nil {
		s.logger.Printf("feed item event skipped mediaID=%s error=%v", id, err)
		return
	}
	s.comments.publishFeedItemCreated(item)
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

func (s *Server) handleShipScores(w http.ResponseWriter, r *http.Request) {
	scores, err := s.scores.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load scores")
		return
	}
	writeJSON(w, http.StatusOK, map[string][]media.GameScore{"scores": scores})
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
	if _, err := s.library.InsertBoardPlaceholder(strings.TrimSuffix(info.Filename, ".board"), info.Name); err != nil {
		s.logger.Printf("board placeholder index failed id=%s name=%q error=%v", info.ID, info.Name, err)
		writeError(w, http.StatusInternalServerError, "failed to index board")
		return
	}
	if info.MediaID != "" {
		s.publishFeedItemCreated(info.MediaID)
	}

	s.logger.Printf("board created id=%s name=%q", info.ID, info.Name)
	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) handleListBoards(w http.ResponseWriter, r *http.Request) {
	boards := s.boards.ListBoards()
	writeJSON(w, http.StatusOK, map[string][]media.BoardInfo{"boards": boards})
}

func (s *Server) handleGetBoard(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	info, err := s.boardInfoForID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	strokes, err := s.boards.Strokes(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	operations, err := s.boards.Operations(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"board":      info,
		"strokes":    strokes,
		"operations": operations,
	})
}

func (s *Server) handleBoardAsset(w http.ResponseWriter, r *http.Request) {
	path, mimeType, err := s.boards.AssetPath(r.PathValue("id"), r.PathValue("assetID"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", mediaCacheControl)
	w.Header().Set("Content-Type", mimeType)
	http.ServeFile(w, r, path)
}

func (s *Server) handleBoardBackground(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, itemErr := s.library.ItemForID(id)
	if itemErr == nil && item.Type == "image" && media.IsBoardBackgroundImageFilename(item.Filename) {
		path, mimeType, err := s.library.PathForID(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", mediaCacheControl)
		w.Header().Set("Content-Type", mimeType)
		http.ServeFile(w, r, path)
		return
	}

	path, mimeType, err := s.boards.BackgroundPath(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", mediaCacheControl)
	w.Header().Set("Content-Type", mimeType)
	http.ServeFile(w, r, path)
}

func (s *Server) handleCreateStroke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var request struct {
		Tool    string      `json:"tool"`
		Points  [][]float64 `json:"points"`
		Color   string      `json:"color"`
		Size    float64     `json:"size"`
		Opacity *float64    `json:"opacity"`
		Author  string      `json:"author"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid stroke payload")
		return
	}

	if _, err := s.boardInfoForID(id); err != nil {
		http.NotFound(w, r)
		return
	}

	opacity := 1.0
	if request.Opacity != nil {
		opacity = *request.Opacity
	}
	stroke, err := s.boards.AddStroke(id, request.Tool, request.Points, request.Color, request.Size, opacity, request.Author)
	if err != nil {
		if errors.Is(err, media.ErrBoardNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.boardHub.publishStroke(id, stroke)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateBoardImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.boardInfoForID(id); err != nil {
		http.NotFound(w, r)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		writeError(w, http.StatusBadRequest, "board image request must be multipart/form-data")
		return
	}
	if r.ContentLength > boardImageMaxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "board image is too large; maximum size is 25 MiB")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, boardImageMaxBytes)
	if err := r.ParseMultipartForm(boardImageMaxBytes); err != nil {
		if isRequestTooLarge(err) {
			writeError(w, http.StatusRequestEntityTooLarge, "board image is too large; maximum size is 25 MiB")
		} else {
			writeError(w, http.StatusBadRequest, "invalid board image payload")
		}
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "board image file is required")
		return
	}
	defer file.Close()
	temp, err := os.CreateTemp("", "feed-ai-board-image-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "board image could not be stored")
		return
	}
	defer os.Remove(temp.Name())
	defer temp.Close()

	header := make([]byte, 512)
	n, readErr := io.ReadFull(file, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		writeError(w, http.StatusBadRequest, "board image could not be read")
		return
	}
	header = header[:n]
	mimeType := http.DetectContentType(header)
	extension := boardImageExtension(mimeType)
	if extension == "" {
		writeError(w, http.StatusBadRequest, "unsupported board image format")
		return
	}
	if _, err := temp.Write(header); err != nil {
		writeError(w, http.StatusInternalServerError, "board image could not be stored")
		return
	}
	if _, err := io.Copy(temp, file); err != nil {
		writeError(w, http.StatusBadRequest, "board image could not be read")
		return
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusInternalServerError, "board image could not be read")
		return
	}

	parseNumber := func(name string) (float64, error) {
		return strconv.ParseFloat(strings.TrimSpace(r.FormValue(name)), 64)
	}
	x, errX := parseNumber("x")
	y, errY := parseNumber("y")
	width, errWidth := parseNumber("width")
	height, errHeight := parseNumber("height")
	rotation, errRotation := parseNumber("rotation")
	if errX != nil || errY != nil || errWidth != nil || errHeight != nil || errRotation != nil {
		writeError(w, http.StatusBadRequest, "invalid board image placement")
		return
	}
	image, err := s.boards.AddImage(id, mimeType, extension, temp, x, y, width, height, rotation, r.FormValue("author"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.boardHub.publishImage(id, image)
	w.WriteHeader(http.StatusNoContent)
}

func boardImageExtension(mimeType string) string {
	switch mimeType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

func (s *Server) boardInfoForID(id string) (media.BoardInfo, error) {
	info, err := s.boards.Get(id)
	if err == nil {
		return info, nil
	}
	if !errors.Is(err, media.ErrBoardNotFound) {
		return media.BoardInfo{}, err
	}

	item, itemErr := s.library.ItemForID(id)
	if itemErr != nil {
		return media.BoardInfo{}, itemErr
	}
	if item.Type != "image" && item.Type != "board" {
		return media.BoardInfo{}, media.ErrBoardNotFound
	}
	if item.Type == "image" && !media.IsBoardBackgroundImageFilename(item.Filename) {
		return media.BoardInfo{}, media.ErrBoardNotFound
	}
	path, mimeType, pathErr := s.library.PathForID(id)
	if pathErr != nil {
		return media.BoardInfo{}, pathErr
	}
	backgroundURL := ""
	if item.Type == "image" {
		backgroundURL = item.URL
	}
	return s.boards.EnsureMediaBoard(id, item.DisplayName, item.Filename, backgroundURL, mimeType, path, item.ModifiedAt)
}

func (s *Server) handleAllBoardEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	ch := s.boardHub.subscribeAll()
	defer s.boardHub.unsubscribeAll(ch)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
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
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		s.logger.Printf(
			"request method=%s clientIP=%q path=%q query=%q%s status=%d requestBytes=%d responseBytes=%d duration=%s",
			r.Method,
			clientIP(r),
			r.URL.Path,
			loggedQuery(r),
			s.mediaRequestLogFields(r),
			recorder.status,
			r.ContentLength,
			recorder.bytes,
			time.Since(started).Round(time.Millisecond),
		)
	})
}

func loggedQuery(r *http.Request) string {
	if r.URL.Path != "/api/ships/socket" || r.URL.Query().Get("resumeToken") == "" {
		return r.URL.RawQuery
	}
	query := r.URL.Query()
	query.Set("resumeToken", "[redacted]")
	return query.Encode()
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (s *Server) mediaRequestLogFields(r *http.Request) string {
	if !strings.HasPrefix(r.URL.Path, "/media/") && !strings.HasPrefix(r.URL.Path, "/api/media/") {
		return ""
	}
	id := r.PathValue("id")
	if id == "" {
		return ""
	}
	item, err := s.library.ItemForID(id)
	if err != nil {
		return fmt.Sprintf(" mediaID=%s", id)
	}
	return fmt.Sprintf(" mediaID=%s filename=%q", id, item.Filename)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wrote {
		return
	}
	r.wrote = true
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(bytes []byte) (int, error) {
	if !r.wrote {
		r.wrote = true
	}
	written, err := r.ResponseWriter.Write(bytes)
	r.bytes += written
	return written, err
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	r.status = http.StatusSwitchingProtocols
	r.wrote = true
	return hijacker.Hijack()
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeUploadTooLarge(w http.ResponseWriter) {
	writeError(w, http.StatusRequestEntityTooLarge, "upload request is too large; maximum upload size is "+uploadMaxSizeLabel)
}

func isRequestTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError) || strings.Contains(err.Error(), "request body too large")
}

func writeDevFallback(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Feed AI</title></head><body><h1>Feed AI API is running</h1><p>Build the frontend with <code>npm --prefix web run build</code> to serve the SPA from Go.</p></body></html>`))
}
