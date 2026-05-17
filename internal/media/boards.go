package media

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const boardsDirName = ".boards"
const boardPointPrecision = 10
const defaultBoardCanvasWidth = 1200
const defaultBoardCanvasHeight = 800

// ErrBoardNotFound is returned when a board ID does not match any existing board.
var ErrBoardNotFound = errors.New("board not found")

// Stroke represents a single drawing stroke on a board.
type Stroke struct {
	ID        string      `json:"id"`
	Tool      string      `json:"tool"`
	Points    [][]float64 `json:"points"`
	Color     string      `json:"color"`
	Size      float64     `json:"size"`
	Author    string      `json:"author"`
	CreatedAt time.Time   `json:"createdAt"`
}

// BoardInfo holds summary information about a board returned to the client.
type BoardInfo struct {
	ID          string           `json:"id"`
	MediaID     string           `json:"mediaId,omitempty"`
	Name        string           `json:"name"`
	Background  *BoardBackground `json:"background,omitempty"`
	Canvas      BoardCanvas      `json:"canvas"`
	StrokeCount int              `json:"strokeCount"`
	CreatedAt   time.Time        `json:"createdAt"`
}

// boardMeta is persisted as the first line of the board JSONL file.
type boardMeta struct {
	Name       string           `json:"name"`
	CreatedAt  time.Time        `json:"createdAt"`
	Background *BoardBackground `json:"background,omitempty"`
	Canvas     BoardCanvas      `json:"canvas,omitempty"`
}

// BoardBackground describes the server-owned visual background for a board.
type BoardBackground struct {
	Type     string `json:"type"`
	Filename string `json:"filename,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	URL      string `json:"url,omitempty"`
}

// BoardCanvas describes the coordinate space used by board strokes.
type BoardCanvas struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// BoardStore manages board JSONL files on disk.
type BoardStore struct {
	root       string // .boards directory
	contentDir string // parent content directory

	mu     sync.RWMutex
	boards map[string]*boardState
}

type boardState struct {
	info    BoardInfo
	strokes []Stroke
}

// NewBoardStore creates a board store rooted at the content directory.
func NewBoardStore(contentRoot string) *BoardStore {
	return &BoardStore{
		root:       filepath.Join(contentRoot, boardsDirName),
		contentDir: contentRoot,
		boards:     make(map[string]*boardState),
	}
}

// Init loads all existing boards from disk.
func (bs *BoardStore) Init() error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if err := os.MkdirAll(bs.root, 0o755); err != nil {
		return fmt.Errorf("create boards directory: %w", err)
	}

	entries, err := os.ReadDir(bs.contentDir)
	if err != nil {
		return fmt.Errorf("read content directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".board") {
			continue
		}
		boardID := strings.TrimSuffix(entry.Name(), ".board")
		if boardID == "" || boardID == "master" {
			continue
		}
		state, loadErr := bs.loadBoardFile(boardID)
		if loadErr != nil {
			info, infoErr := entry.Info()
			if infoErr != nil {
				continue
			}
			state, loadErr = bs.createBoardFileForPlaceholder(boardID, info.ModTime().UTC())
			if loadErr != nil {
				continue
			}
		}
		bs.boards[boardID] = state
	}

	if state, err := bs.loadBoardFile("master"); err == nil {
		bs.boards["master"] = state
	}

	// Ensure master board exists.
	if _, ok := bs.boards["master"]; !ok {
		now := time.Now().UTC()
		name := "Master Board"
		meta := boardMeta{Name: name, CreatedAt: now, Background: defaultBoardBackground(), Canvas: defaultBoardCanvas()}
		metaLine, _ := json.Marshal(meta)

		filePath := bs.boardFilePath("master")
		if err := os.WriteFile(filePath, append(metaLine, '\n'), 0o644); err == nil {
			bs.boards["master"] = &boardState{
				info: BoardInfo{
					ID:          "master",
					Name:        name,
					Background:  defaultBoardBackground(),
					Canvas:      defaultBoardCanvas(),
					StrokeCount: 0,
					CreatedAt:   now,
				},
			}
		}
	}

	return nil
}

// Create creates a new board and returns its info.
func (bs *BoardStore) Create(name string) (BoardInfo, error) {
	id := generateBoardID()
	now := time.Now().UTC()

	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultBoardName(id)
	}

	meta := boardMeta{Name: name, CreatedAt: now, Background: defaultBoardBackground(), Canvas: defaultBoardCanvas()}
	metaLine, err := json.Marshal(meta)
	if err != nil {
		return BoardInfo{}, fmt.Errorf("marshal board meta: %w", err)
	}

	if err := os.MkdirAll(bs.root, 0o755); err != nil {
		return BoardInfo{}, fmt.Errorf("create boards directory: %w", err)
	}

	filePath := bs.boardFilePath(id)
	if err := os.WriteFile(filePath, append(metaLine, '\n'), 0o644); err != nil {
		return BoardInfo{}, fmt.Errorf("create board file: %w", err)
	}

	info := BoardInfo{
		ID:          id,
		MediaID:     EncodeID(id + ".board"),
		Name:        name,
		Background:  defaultBoardBackground(),
		Canvas:      defaultBoardCanvas(),
		StrokeCount: 0,
		CreatedAt:   now,
	}

	bs.mu.Lock()
	bs.boards[id] = &boardState{info: info, strokes: nil}
	bs.mu.Unlock()

	_ = bs.ensureBoardPlaceholder(id, now)

	return info, nil
}

// CreateWithImageBackground creates a board whose background is a server-owned
// copy of the uploaded image stored under .boards.
func (bs *BoardStore) CreateWithImageBackground(name string, originalName string, reader io.Reader, sourceModifiedAt time.Time) (BoardInfo, error) {
	return bs.createImageBackgroundBoard(name, originalName, reader, sourceModifiedAt, time.Time{})
}

// CreateFromImageFile converts an existing image file into a board background
// and removes the original image after the board is durably created.
func (bs *BoardStore) CreateFromImageFile(name string, path string, sourceModifiedAt time.Time) (BoardInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return BoardInfo{}, err
	}
	defer file.Close()

	info, err := bs.createImageBackgroundBoard(name, filepath.Base(path), file, sourceModifiedAt, sourceModifiedAt)
	if err != nil {
		return BoardInfo{}, err
	}
	if err := os.Remove(path); err != nil {
		return BoardInfo{}, err
	}
	return info, nil
}

func (bs *BoardStore) createImageBackgroundBoard(name string, originalName string, reader io.Reader, backgroundModifiedAt time.Time, placeholderModifiedAt time.Time) (BoardInfo, error) {
	if originalName == "" {
		return BoardInfo{}, errors.New("filename is required")
	}
	if filepath.Base(originalName) != originalName || strings.ContainsAny(originalName, `/\`) {
		return BoardInfo{}, errors.New("filename must not include a path")
	}
	extension := strings.ToLower(filepath.Ext(originalName))
	kind, ok := kindForPath(originalName)
	if !ok || kind != "image" {
		return BoardInfo{}, fmt.Errorf("unsupported image type %q", extension)
	}

	id := generateBoardID()
	now := time.Now().UTC()
	if backgroundModifiedAt.IsZero() {
		backgroundModifiedAt = now
	} else {
		backgroundModifiedAt = backgroundModifiedAt.UTC()
	}
	if placeholderModifiedAt.IsZero() {
		placeholderModifiedAt = now
	} else {
		placeholderModifiedAt = placeholderModifiedAt.UTC()
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = originalName
	}

	if err := os.MkdirAll(bs.root, 0o755); err != nil {
		return BoardInfo{}, fmt.Errorf("create boards directory: %w", err)
	}

	backgroundFilename := id + "_bgimg" + extension
	backgroundPath := filepath.Join(bs.root, backgroundFilename)
	backgroundFile, err := os.OpenFile(backgroundPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return BoardInfo{}, fmt.Errorf("create board background file: %w", err)
	}
	size, copyErr := io.Copy(backgroundFile, reader)
	closeErr := backgroundFile.Close()
	if copyErr != nil {
		_ = os.Remove(backgroundPath)
		return BoardInfo{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(backgroundPath)
		return BoardInfo{}, closeErr
	}
	if size == 0 {
		_ = os.Remove(backgroundPath)
		return BoardInfo{}, errors.New("uploaded file is empty")
	}
	if err := os.Chtimes(backgroundPath, backgroundModifiedAt, backgroundModifiedAt); err != nil {
		_ = os.Remove(backgroundPath)
		return BoardInfo{}, err
	}

	canvas := imageCanvas(backgroundPath)
	background := &BoardBackground{
		Type:     "image",
		Filename: backgroundFilename,
		MimeType: mimeType(backgroundPath),
		URL:      "/api/boards/" + id + "/background",
	}
	meta := boardMeta{Name: name, CreatedAt: now, Background: background, Canvas: canvas}
	metaLine, err := json.Marshal(meta)
	if err != nil {
		_ = os.Remove(backgroundPath)
		return BoardInfo{}, fmt.Errorf("marshal board meta: %w", err)
	}
	if err := os.WriteFile(bs.boardFilePath(id), append(metaLine, '\n'), 0o644); err != nil {
		_ = os.Remove(backgroundPath)
		return BoardInfo{}, fmt.Errorf("create board file: %w", err)
	}

	info := BoardInfo{
		ID:          id,
		MediaID:     EncodeID(id + ".board"),
		Name:        name,
		Background:  background,
		Canvas:      canvas,
		StrokeCount: 0,
		CreatedAt:   now,
	}

	bs.mu.Lock()
	bs.boards[id] = &boardState{info: info, strokes: nil}
	bs.mu.Unlock()

	_ = bs.ensureBoardPlaceholder(id, placeholderModifiedAt)
	return info, nil
}

// Get returns the board info for the given ID.
func (bs *BoardStore) Get(id string) (BoardInfo, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	state, ok := bs.boards[id]
	if !ok {
		return BoardInfo{}, ErrBoardNotFound
	}
	return state.info, nil
}

// ListBoards returns all board infos, newest first.
func (bs *BoardStore) ListBoards() []BoardInfo {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	boards := make([]BoardInfo, 0, len(bs.boards))
	for _, state := range bs.boards {
		boards = append(boards, state.info)
	}
	sort.Slice(boards, func(i, j int) bool {
		if !boards[i].CreatedAt.Equal(boards[j].CreatedAt) {
			return boards[i].CreatedAt.After(boards[j].CreatedAt)
		}
		return boards[i].ID < boards[j].ID
	})
	return boards
}

// Strokes returns all strokes for a board.
func (bs *BoardStore) Strokes(id string) ([]Stroke, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	state, ok := bs.boards[id]
	if !ok {
		return nil, ErrBoardNotFound
	}

	strokes := make([]Stroke, len(state.strokes))
	copy(strokes, state.strokes)
	return strokes, nil
}

// BackgroundPath returns a safe server-owned background file for a board.
func (bs *BoardStore) BackgroundPath(id string) (string, string, error) {
	bs.mu.RLock()
	state, ok := bs.boards[id]
	bs.mu.RUnlock()
	if !ok {
		return "", "", ErrBoardNotFound
	}
	if state.info.Background == nil || state.info.Background.Type != "image" || strings.TrimSpace(state.info.Background.Filename) == "" {
		return "", "", os.ErrNotExist
	}

	filename := state.info.Background.Filename
	if filepath.Base(filename) != filename || strings.ContainsAny(filename, `/\`) {
		return "", "", errors.New("invalid board background filename")
	}
	path := filepath.Join(bs.root, filename)
	mime := state.info.Background.MimeType
	if mime == "" {
		mime = mimeType(path)
	}
	return path, mime, nil
}

// AddStroke appends a stroke to the board file and updates the in-memory state.
func (bs *BoardStore) AddStroke(id string, tool string, points [][]float64, color string, size float64, author string) (Stroke, error) {
	tool = strings.TrimSpace(tool)
	if tool != "freeform" && tool != "line" {
		return Stroke{}, errors.New("invalid tool: must be freeform or line")
	}
	if len(points) == 0 {
		return Stroke{}, errors.New("stroke must have at least 1 point")
	}
	if tool == "line" && len(points) < 2 {
		return Stroke{}, errors.New("line stroke must have at least 2 points")
	}
	normalizedPoints, err := normalizeStrokePoints(points)
	if err != nil {
		return Stroke{}, err
	}
	color = strings.TrimSpace(color)
	if color == "" {
		color = "#ffffff"
	}
	if size <= 0 {
		size = 3
	}
	author = strings.TrimSpace(author)
	if author == "" {
		author = "Guest"
	}

	bs.mu.RLock()
	_, ok := bs.boards[id]
	bs.mu.RUnlock()
	if !ok {
		return Stroke{}, ErrBoardNotFound
	}

	stroke := Stroke{
		ID:        generateStrokeID(),
		Tool:      tool,
		Points:    normalizedPoints,
		Color:     color,
		Size:      size,
		Author:    author,
		CreatedAt: time.Now().UTC(),
	}

	line, err := json.Marshal(stroke)
	if err != nil {
		return Stroke{}, fmt.Errorf("marshal stroke: %w", err)
	}

	filePath := bs.boardFilePath(id)
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return Stroke{}, fmt.Errorf("open board file: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
		return Stroke{}, fmt.Errorf("write stroke: %w", err)
	}

	bs.mu.Lock()
	state := bs.boards[id]
	state.strokes = append(state.strokes, stroke)
	state.info.StrokeCount = len(state.strokes)
	bs.mu.Unlock()

	return stroke, nil
}

func normalizeStrokePoints(points [][]float64) ([][]float64, error) {
	normalized := make([][]float64, len(points))
	for i, point := range points {
		if len(point) != 2 {
			return nil, errors.New("stroke points must be [x,y] coordinate pairs")
		}
		x, err := normalizeBoardCoordinate(point[0])
		if err != nil {
			return nil, err
		}
		y, err := normalizeBoardCoordinate(point[1])
		if err != nil {
			return nil, err
		}
		normalized[i] = []float64{x, y}
	}
	return normalized, nil
}

func normalizeBoardCoordinate(value float64) (float64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errors.New("stroke coordinates must be finite numbers")
	}
	return math.Round(value*boardPointPrecision) / boardPointPrecision, nil
}

func (bs *BoardStore) boardFilePath(id string) string {
	return filepath.Join(bs.root, id+".jsonl")
}

func (bs *BoardStore) loadBoardFile(id string) (*boardState, error) {
	filePath := bs.boardFilePath(id)
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// First line is meta
	if !scanner.Scan() {
		return nil, errors.New("empty board file")
	}

	var meta boardMeta
	if err := json.Unmarshal(scanner.Bytes(), &meta); err != nil {
		return nil, fmt.Errorf("parse board meta: %w", err)
	}
	if meta.Background == nil {
		meta.Background = defaultBoardBackground()
	}
	if meta.Canvas.Width <= 0 || meta.Canvas.Height <= 0 {
		meta.Canvas = defaultBoardCanvas()
	}
	if meta.Background.Type == "image" && meta.Background.URL == "" {
		meta.Background.URL = "/api/boards/" + id + "/background"
	}
	if meta.Background.Type == "image" && meta.Background.Filename != "" && filepath.Base(meta.Background.Filename) == meta.Background.Filename {
		canvas := imageCanvas(filepath.Join(bs.root, meta.Background.Filename))
		if canvas.Width > 0 && canvas.Height > 0 {
			meta.Canvas = canvas
		}
	}

	var strokes []Stroke
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var stroke Stroke
		if err := json.Unmarshal(line, &stroke); err != nil {
			continue // Skip malformed lines
		}
		strokes = append(strokes, stroke)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan board file: %w", err)
	}

	return &boardState{
		info: BoardInfo{
			ID:          id,
			MediaID:     boardMediaID(id),
			Name:        meta.Name,
			Background:  meta.Background,
			Canvas:      meta.Canvas,
			StrokeCount: len(strokes),
			CreatedAt:   meta.CreatedAt,
		},
		strokes: strokes,
	}, nil
}

func (bs *BoardStore) createBoardFileForPlaceholder(id string, createdAt time.Time) (*boardState, error) {
	name := defaultBoardName(id)
	meta := boardMeta{Name: name, CreatedAt: createdAt, Background: defaultBoardBackground(), Canvas: defaultBoardCanvas()}
	metaLine, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("marshal board meta: %w", err)
	}
	if err := os.WriteFile(bs.boardFilePath(id), append(metaLine, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("create board file: %w", err)
	}
	return &boardState{
		info: BoardInfo{
			ID:          id,
			MediaID:     boardMediaID(id),
			Name:        name,
			Background:  defaultBoardBackground(),
			Canvas:      defaultBoardCanvas(),
			StrokeCount: 0,
			CreatedAt:   createdAt,
		},
	}, nil
}

func generateBoardID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("generate board id: %v", err))
	}
	return hex.EncodeToString(b)
}

func defaultBoardName(id string) string {
	if len(id) >= 6 {
		return "Board " + id[:6]
	}
	if id != "" {
		return "Board " + id
	}
	return "Board"
}

func generateStrokeID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("generate stroke id: %v", err))
	}
	return hex.EncodeToString(b)
}

func boardMediaID(id string) string {
	if id == "master" {
		return ""
	}
	return EncodeID(id + ".board")
}

func (bs *BoardStore) ensureBoardPlaceholder(id string, modifiedAt time.Time) error {
	placeholderPath := filepath.Join(bs.contentDir, id+".board")
	if _, err := os.Stat(placeholderPath); err == nil {
		if !modifiedAt.IsZero() {
			_ = os.Chtimes(placeholderPath, modifiedAt, modifiedAt)
		}
		return nil
	}
	if err := os.WriteFile(placeholderPath, []byte{}, 0o644); err != nil {
		return err
	}
	if !modifiedAt.IsZero() {
		_ = os.Chtimes(placeholderPath, modifiedAt, modifiedAt)
	}
	return nil
}

func defaultBoardBackground() *BoardBackground {
	return &BoardBackground{Type: "default"}
}

func defaultBoardCanvas() BoardCanvas {
	return BoardCanvas{Width: defaultBoardCanvasWidth, Height: defaultBoardCanvasHeight}
}

func imageCanvas(path string) BoardCanvas {
	file, err := os.Open(path)
	if err != nil {
		return defaultBoardCanvas()
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return defaultBoardCanvas()
	}
	if jpegOrientationSwapsAxes(path) {
		return BoardCanvas{Width: config.Height, Height: config.Width}
	}
	return BoardCanvas{Width: config.Width, Height: config.Height}
}

func jpegOrientationSwapsAxes(path string) bool {
	orientation, ok := jpegEXIFOrientation(path)
	return ok && orientation >= 5 && orientation <= 8
}

func jpegEXIFOrientation(path string) (int, bool) {
	file, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer file.Close()

	var header [2]byte
	if _, err := io.ReadFull(file, header[:]); err != nil || header != [2]byte{0xff, 0xd8} {
		return 0, false
	}

	for {
		marker, err := nextJPEGMarker(file)
		if err != nil {
			return 0, false
		}
		if marker == 0xda || marker == 0xd9 {
			return 0, false
		}

		var lengthBytes [2]byte
		if _, err := io.ReadFull(file, lengthBytes[:]); err != nil {
			return 0, false
		}
		length := int(binary.BigEndian.Uint16(lengthBytes[:]))
		if length < 2 {
			return 0, false
		}
		payloadLength := length - 2
		payload := make([]byte, payloadLength)
		if _, err := io.ReadFull(file, payload); err != nil {
			return 0, false
		}
		if marker == 0xe1 && len(payload) > 6 && string(payload[:6]) == "Exif\x00\x00" {
			return parseEXIFOrientation(payload[6:])
		}
	}
}

func nextJPEGMarker(r io.Reader) (byte, error) {
	var b [1]byte
	for {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, err
		}
		if b[0] == 0xff {
			break
		}
	}
	for {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, err
		}
		if b[0] != 0xff {
			return b[0], nil
		}
	}
}

func parseEXIFOrientation(tiff []byte) (int, bool) {
	if len(tiff) < 8 {
		return 0, false
	}

	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0, false
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return 0, false
	}

	ifdOffset := int(order.Uint32(tiff[4:8]))
	if ifdOffset < 0 || ifdOffset+2 > len(tiff) {
		return 0, false
	}
	entryCount := int(order.Uint16(tiff[ifdOffset : ifdOffset+2]))
	entriesStart := ifdOffset + 2
	for i := 0; i < entryCount; i++ {
		entryStart := entriesStart + i*12
		if entryStart+12 > len(tiff) {
			return 0, false
		}
		entry := tiff[entryStart : entryStart+12]
		tag := order.Uint16(entry[0:2])
		fieldType := order.Uint16(entry[2:4])
		count := order.Uint32(entry[4:8])
		if tag != 0x0112 || fieldType != 3 || count != 1 {
			continue
		}
		return int(order.Uint16(entry[8:10])), true
	}
	return 0, false
}
