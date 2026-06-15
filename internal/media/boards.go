package media

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
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
	"net/http"
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
const boardAssetsDirName = "assets"
const boardStickerPackDirName = "sticker-pack"

// ErrBoardNotFound is returned when a board ID does not match any existing board.
var ErrBoardNotFound = errors.New("board not found")

// ErrBoardAssetNotFound is returned when an asset ID is not registered.
var ErrBoardAssetNotFound = errors.New("board asset not found")

// Stroke represents a single drawing stroke on a board.
type Stroke struct {
	ID        string      `json:"id"`
	Tool      string      `json:"tool"`
	Points    [][]float64 `json:"points"`
	Color     string      `json:"color"`
	Size      float64     `json:"size"`
	Opacity   float64     `json:"opacity"`
	Author    string      `json:"author"`
	CreatedAt time.Time   `json:"createdAt"`
}

// StrokeInput describes a stroke before the server assigns identity and time.
type StrokeInput struct {
	Tool    string      `json:"tool"`
	Points  [][]float64 `json:"points"`
	Color   string      `json:"color"`
	Size    float64     `json:"size"`
	Opacity *float64    `json:"opacity"`
	Author  string      `json:"author"`
}

// BoardImageInput describes a placement of an existing board asset.
type BoardImageInput struct {
	AssetID  string  `json:"assetId"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	Rotation float64 `json:"rotation"`
	FlipX    bool    `json:"flipX"`
	Author   string  `json:"author"`
}

// BoardOperationInput describes an operation before the server assigns identity and time.
type BoardOperationInput struct {
	Type   string           `json:"type"`
	Stroke *StrokeInput     `json:"stroke,omitempty"`
	Image  *BoardImageInput `json:"image,omitempty"`
}

// BoardImage represents an image fixed into the board operation history.
type BoardImage struct {
	ID        string    `json:"id"`
	AssetID   string    `json:"assetId"`
	URL       string    `json:"url"`
	MimeType  string    `json:"mimeType"`
	X         float64   `json:"x"`
	Y         float64   `json:"y"`
	Width     float64   `json:"width"`
	Height    float64   `json:"height"`
	Rotation  float64   `json:"rotation"`
	FlipX     bool      `json:"flipX"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	Filename  string    `json:"assetFilename,omitempty"`
}

// BoardAsset describes one reusable image stored by the board system.
type BoardAsset struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	MimeType   string    `json:"mimeType"`
	UsageCount int       `json:"usageCount"`
	CreatedAt  time.Time `json:"createdAt"`
	Filename   string    `json:"-"`
}

// BoardOperation preserves the layer order of strokes and fixed images.
type BoardOperation struct {
	Type   string      `json:"type"`
	Stroke *Stroke     `json:"stroke,omitempty"`
	Image  *BoardImage `json:"image,omitempty"`
}

// BoardInfo holds summary information about a board returned to the client.
type BoardInfo struct {
	ID          string           `json:"id"`
	MediaID     string           `json:"mediaId,omitempty"`
	Filename    string           `json:"filename,omitempty"`
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
	assets map[string]BoardAsset
}

type boardState struct {
	info       BoardInfo
	strokes    []Stroke
	operations []BoardOperation
	filePath   string
}

// NewBoardStore creates a board store rooted at the content directory.
func NewBoardStore(contentRoot string) *BoardStore {
	return &BoardStore{
		root:       filepath.Join(contentRoot, boardsDirName),
		contentDir: contentRoot,
		boards:     make(map[string]*boardState),
		assets:     make(map[string]BoardAsset),
	}
}

// Init loads all existing boards from disk.
func (bs *BoardStore) Init() error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if err := os.MkdirAll(bs.root, 0o755); err != nil {
		return fmt.Errorf("create boards directory: %w", err)
	}
	if err := bs.importStickerPack(); err != nil {
		return fmt.Errorf("import board sticker pack: %w", err)
	}

	entries, err := os.ReadDir(bs.contentDir)
	if err != nil {
		return fmt.Errorf("read content directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".board") {
			continue
		}
		boardName := strings.TrimSuffix(entry.Name(), ".board")
		if boardName == "" || boardName == "master" {
			continue
		}
		filename := entry.Name()
		mediaID := EncodeID(filename)
		state, loadErr := bs.loadBoardFile(mediaID, filename)
		if loadErr != nil {
			info, infoErr := entry.Info()
			if infoErr != nil {
				continue
			}
			state, loadErr = bs.createBoardFileForPlaceholder(mediaID, filename, info.ModTime().UTC())
			if loadErr != nil {
				continue
			}
		}
		bs.boards[mediaID] = state
	}

	if state, err := bs.loadBoardFile("master", "master"); err == nil {
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
				filePath: filePath,
				info: BoardInfo{
					ID:          "master",
					Filename:    "master",
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
	now := time.Now().UTC()

	name = strings.TrimSpace(name)
	if name == "" {
		name = "board"
	}
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return BoardInfo{}, errors.New("board name must not include a path")
	}
	if err := os.MkdirAll(bs.contentDir, 0o755); err != nil {
		return BoardInfo{}, fmt.Errorf("create content directory: %w", err)
	}
	root, err := filepath.Abs(bs.contentDir)
	if err != nil {
		return BoardInfo{}, err
	}
	filename, placeholderFile, err := createUniqueFile(root, name+".board")
	if err != nil {
		return BoardInfo{}, fmt.Errorf("create board placeholder: %w", err)
	}
	if err := placeholderFile.Close(); err != nil {
		_ = os.Remove(filepath.Join(root, filename))
		return BoardInfo{}, fmt.Errorf("close board placeholder: %w", err)
	}
	if err := os.Chtimes(filepath.Join(root, filename), now, now); err != nil {
		_ = os.Remove(filepath.Join(root, filename))
		return BoardInfo{}, err
	}
	name = strings.TrimSuffix(filename, ".board")
	mediaID := EncodeID(filename)

	meta := boardMeta{Name: name, CreatedAt: now, Background: defaultBoardBackground(), Canvas: defaultBoardCanvas()}
	metaLine, err := json.Marshal(meta)
	if err != nil {
		_ = os.Remove(filepath.Join(root, filename))
		return BoardInfo{}, fmt.Errorf("marshal board meta: %w", err)
	}

	if err := os.MkdirAll(bs.root, 0o755); err != nil {
		_ = os.Remove(filepath.Join(root, filename))
		return BoardInfo{}, fmt.Errorf("create boards directory: %w", err)
	}

	filePath := bs.boardFilePath(filename)
	if err := os.WriteFile(filePath, append(metaLine, '\n'), 0o644); err != nil {
		_ = os.Remove(filepath.Join(root, filename))
		return BoardInfo{}, fmt.Errorf("create board file: %w", err)
	}

	info := BoardInfo{
		ID:          mediaID,
		MediaID:     mediaID,
		Filename:    filename,
		Name:        name,
		Background:  defaultBoardBackground(),
		Canvas:      defaultBoardCanvas(),
		StrokeCount: 0,
		CreatedAt:   now,
	}

	bs.mu.Lock()
	bs.boards[mediaID] = &boardState{info: info, strokes: nil, filePath: filePath}
	bs.mu.Unlock()

	return info, nil
}

// EnsureMediaBoard returns the drawing history board associated with a media
// item, creating its JSONL history file when it does not exist yet.
func (bs *BoardStore) EnsureMediaBoard(mediaID string, name string, filename string, backgroundURL string, mimeType string, mediaPath string, createdAt time.Time) (BoardInfo, error) {
	if mediaID != "master" {
		if err := validateMediaID(mediaID); err != nil {
			return BoardInfo{}, err
		}
	}
	if strings.TrimSpace(filename) == "" {
		return BoardInfo{}, errors.New("filename is required")
	}
	if filename != filepath.Base(filename) || strings.ContainsAny(filename, `/\`) {
		return BoardInfo{}, errors.New("filename must not include a path")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = filename
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	} else {
		createdAt = createdAt.UTC()
	}
	background := &BoardBackground{
		Filename: filename,
		MimeType: mimeType,
		URL:      backgroundURL,
	}
	canvas := defaultBoardCanvas()
	if backgroundURL != "" {
		background.Type = "image"
		canvas = imageCanvas(mediaPath)
		if canvas.Width <= 0 || canvas.Height <= 0 {
			canvas = defaultBoardCanvas()
		}
	} else {
		background.Type = "default"
	}
	filePath := bs.boardFilePath(filename)

	bs.mu.RLock()
	if state, ok := bs.boards[mediaID]; ok {
		strokeCount := len(state.strokes)
		bs.mu.RUnlock()
		bs.mu.Lock()
		state.info.ID = mediaID
		state.info.MediaID = mediaID
		state.info.Filename = filename
		state.info.Name = name
		state.info.Background = background
		state.info.Canvas = canvas
		state.info.StrokeCount = strokeCount
		state.filePath = filePath
		info := state.info
		bs.mu.Unlock()
		return info, nil
	}
	bs.mu.RUnlock()

	if state, err := bs.loadBoardFile(mediaID, filename); err == nil {
		bs.mu.Lock()
		state.info.ID = mediaID
		state.info.MediaID = mediaID
		state.info.Filename = filename
		state.info.Name = name
		state.info.Background = background
		state.info.Canvas = canvas
		state.filePath = filePath
		bs.boards[mediaID] = state
		info := state.info
		bs.mu.Unlock()
		return info, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return BoardInfo{}, err
	}

	if err := os.MkdirAll(bs.root, 0o755); err != nil {
		return BoardInfo{}, fmt.Errorf("create boards directory: %w", err)
	}
	meta := boardMeta{Name: name, CreatedAt: createdAt, Background: background, Canvas: canvas}
	metaLine, err := json.Marshal(meta)
	if err != nil {
		return BoardInfo{}, fmt.Errorf("marshal board meta: %w", err)
	}
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return bs.EnsureMediaBoard(mediaID, name, filename, backgroundURL, mimeType, mediaPath, createdAt)
		}
		return BoardInfo{}, fmt.Errorf("create media board file: %w", err)
	}
	if _, err := f.Write(append(metaLine, '\n')); err != nil {
		_ = f.Close()
		_ = os.Remove(filePath)
		return BoardInfo{}, fmt.Errorf("write media board file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(filePath)
		return BoardInfo{}, fmt.Errorf("close media board file: %w", err)
	}

	info := BoardInfo{
		ID:          mediaID,
		MediaID:     mediaID,
		Filename:    filename,
		Name:        name,
		Background:  background,
		Canvas:      canvas,
		StrokeCount: 0,
		CreatedAt:   createdAt,
	}
	bs.mu.Lock()
	bs.boards[mediaID] = &boardState{info: info, strokes: nil, filePath: filePath}
	bs.mu.Unlock()
	return info, nil
}

// Get returns the board info for the given media ID.
func (bs *BoardStore) Get(mediaID string) (BoardInfo, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	state, ok := bs.boards[mediaID]
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

// LatestChangeTimes returns the time of the final persisted operation for each board.
func (bs *BoardStore) LatestChangeTimes() map[string]time.Time {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	result := make(map[string]time.Time)
	for mediaID, state := range bs.boards {
		for i := len(state.operations) - 1; i >= 0; i-- {
			updatedAt := boardOperationTime(state.operations[i])
			if !updatedAt.IsZero() {
				result[mediaID] = updatedAt
				break
			}
		}
	}
	return result
}

func boardOperationTime(operation BoardOperation) time.Time {
	if operation.Stroke != nil {
		return operation.Stroke.CreatedAt
	}
	if operation.Image != nil {
		return operation.Image.CreatedAt
	}
	return time.Time{}
}

// Strokes returns all strokes for a board.
func (bs *BoardStore) Strokes(mediaID string) ([]Stroke, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	state, ok := bs.boards[mediaID]
	if !ok {
		return nil, ErrBoardNotFound
	}

	strokes := make([]Stroke, len(state.strokes))
	copy(strokes, state.strokes)
	return strokes, nil
}

// Operations returns the ordered board history.
func (bs *BoardStore) Operations(mediaID string) ([]BoardOperation, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	state, ok := bs.boards[mediaID]
	if !ok {
		return nil, ErrBoardNotFound
	}
	operations := make([]BoardOperation, len(state.operations))
	copy(operations, state.operations)
	return operations, nil
}

// AssetPath returns a safe server-owned fixed image asset.
func (bs *BoardStore) AssetPath(mediaID, assetID string) (string, string, error) {
	if assetID == "" || filepath.Base(assetID) != assetID {
		return "", "", ErrBoardNotFound
	}
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	state, ok := bs.boards[mediaID]
	if !ok {
		return "", "", ErrBoardNotFound
	}
	for _, operation := range state.operations {
		if operation.Image != nil && operation.Image.AssetID == assetID {
			return filepath.Join(bs.root, boardAssetsDirName, operation.Image.Filename), operation.Image.MimeType, nil
		}
	}
	return "", "", ErrBoardNotFound
}

// Assets returns unique reusable image assets, newest first.
func (bs *BoardStore) Assets() []BoardAsset {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	assets := make(map[string]BoardAsset, len(bs.assets))
	for assetID, asset := range bs.assets {
		assets[assetID] = asset
	}
	for _, state := range bs.boards {
		for _, operation := range state.operations {
			if operation.Image == nil {
				continue
			}
			image := operation.Image
			asset, ok := assets[image.AssetID]
			if !ok {
				asset = BoardAsset{
					ID:        image.AssetID,
					URL:       "/api/board-assets/" + image.AssetID,
					MimeType:  image.MimeType,
					CreatedAt: image.CreatedAt,
					Filename:  image.Filename,
				}
			}
			asset.UsageCount++
			if image.CreatedAt.After(asset.CreatedAt) {
				asset.CreatedAt = image.CreatedAt
			}
			assets[image.AssetID] = asset
		}
	}

	result := make([]BoardAsset, 0, len(assets))
	for _, asset := range assets {
		result = append(result, asset)
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// GlobalAssetPath returns an asset referenced by any board.
func (bs *BoardStore) GlobalAssetPath(assetID string) (string, string, error) {
	if assetID == "" || filepath.Base(assetID) != assetID {
		return "", "", ErrBoardAssetNotFound
	}
	for _, asset := range bs.Assets() {
		if asset.ID == assetID && filepath.Base(asset.Filename) == asset.Filename {
			return filepath.Join(bs.root, boardAssetsDirName, asset.Filename), asset.MimeType, nil
		}
	}
	return "", "", ErrBoardAssetNotFound
}

// AddAsset stores and registers a reusable image without placing it on a board.
func (bs *BoardStore) AddAsset(mimeType string, source io.Reader) (BoardAsset, error) {
	if !isBoardImageMimeType(mimeType) {
		return BoardAsset{}, errors.New("unsupported board image format")
	}
	assetID, filename, err := bs.storeBoardAsset(source)
	if err != nil {
		return BoardAsset{}, err
	}

	bs.mu.Lock()
	defer bs.mu.Unlock()
	if asset, ok := bs.assets[assetID]; ok {
		return asset, nil
	}
	asset := BoardAsset{
		ID:        assetID,
		URL:       "/api/board-assets/" + assetID,
		MimeType:  mimeType,
		CreatedAt: time.Now().UTC(),
		Filename:  filename,
	}
	bs.assets[assetID] = asset
	return asset, nil
}

// BackgroundPath returns a safe server-owned background file for a board.
func (bs *BoardStore) BackgroundPath(mediaID string) (string, string, error) {
	bs.mu.RLock()
	state, ok := bs.boards[mediaID]
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
func (bs *BoardStore) AddStroke(mediaID string, tool string, points [][]float64, color string, size float64, opacity float64, author string) (Stroke, error) {
	strokes, err := bs.AddStrokes(mediaID, []StrokeInput{{
		Tool: tool, Points: points, Color: color, Size: size, Opacity: &opacity, Author: author,
	}})
	if err != nil {
		return Stroke{}, err
	}
	return strokes[0], nil
}

// AddStrokes appends a validated group of strokes in order.
func (bs *BoardStore) AddStrokes(mediaID string, inputs []StrokeInput) ([]Stroke, error) {
	if len(inputs) == 0 {
		return nil, errors.New("at least one stroke is required")
	}

	operationInputs := make([]BoardOperationInput, 0, len(inputs))
	for i := range inputs {
		operationInputs = append(operationInputs, BoardOperationInput{Type: "stroke", Stroke: &inputs[i]})
	}
	operations, err := bs.AddOperations(mediaID, operationInputs)
	if err != nil {
		return nil, err
	}
	strokes := make([]Stroke, 0, len(operations))
	for _, operation := range operations {
		strokes = append(strokes, *operation.Stroke)
	}
	return strokes, nil
}

// AddOperations validates and appends a mixed group of strokes and existing assets in order.
func (bs *BoardStore) AddOperations(mediaID string, inputs []BoardOperationInput) ([]BoardOperation, error) {
	if len(inputs) == 0 {
		return nil, errors.New("at least one board operation is required")
	}

	bs.mu.RLock()
	state, ok := bs.boards[mediaID]
	bs.mu.RUnlock()
	if !ok {
		return nil, ErrBoardNotFound
	}

	operations := make([]BoardOperation, 0, len(inputs))
	var lines strings.Builder
	for _, input := range inputs {
		var operation BoardOperation
		switch {
		case input.Type == "stroke" && input.Stroke != nil:
			opacity := 1.0
			if input.Stroke.Opacity != nil {
				opacity = *input.Stroke.Opacity
			}
			stroke, err := newStroke(input.Stroke.Tool, input.Stroke.Points, input.Stroke.Color, input.Stroke.Size, opacity, input.Stroke.Author)
			if err != nil {
				return nil, err
			}
			operation = BoardOperation{Type: "stroke", Stroke: &stroke}
		case input.Type == "image" && input.Image != nil:
			path, mimeType, err := bs.GlobalAssetPath(input.Image.AssetID)
			if err != nil {
				return nil, err
			}
			image, err := bs.newBoardImage(
				mediaID,
				input.Image.AssetID,
				filepath.Base(path),
				mimeType,
				input.Image.X,
				input.Image.Y,
				input.Image.Width,
				input.Image.Height,
				input.Image.Rotation,
				input.Image.FlipX,
				input.Image.Author,
			)
			if err != nil {
				return nil, err
			}
			operation = BoardOperation{Type: "image", Image: &image}
		default:
			return nil, errors.New("invalid board operation")
		}

		line, err := json.Marshal(operation)
		if err != nil {
			return nil, fmt.Errorf("marshal board operation: %w", err)
		}
		lines.Write(line)
		lines.WriteByte('\n')
		operations = append(operations, operation)
	}

	f, err := os.OpenFile(state.filePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open board file: %w", err)
	}
	if _, err := io.WriteString(f, lines.String()); err != nil {
		f.Close()
		return nil, fmt.Errorf("write strokes: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("close board file: %w", err)
	}

	bs.mu.Lock()
	state = bs.boards[mediaID]
	state.operations = append(state.operations, operations...)
	for _, operation := range operations {
		if operation.Stroke != nil {
			state.strokes = append(state.strokes, *operation.Stroke)
		}
	}
	state.info.StrokeCount = len(state.strokes)
	bs.mu.Unlock()

	return operations, nil
}

func newStroke(tool string, points [][]float64, color string, size float64, opacity float64, author string) (Stroke, error) {
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
	if math.IsNaN(opacity) || math.IsInf(opacity, 0) || opacity <= 0 || opacity > 1 {
		return Stroke{}, errors.New("stroke opacity must be greater than 0 and at most 1")
	}
	author = strings.TrimSpace(author)
	if author == "" {
		author = "Guest"
	}

	stroke := Stroke{
		ID:        generateStrokeID(),
		Tool:      tool,
		Points:    normalizedPoints,
		Color:     color,
		Size:      size,
		Opacity:   opacity,
		Author:    author,
		CreatedAt: time.Now().UTC(),
	}

	return stroke, nil
}

// AddImage stores an image asset and appends its placement to board history.
func (bs *BoardStore) AddImage(mediaID, mimeType string, source io.Reader, x, y, width, height, rotation float64, flipX bool, author string) (BoardImage, error) {
	if _, _, _, _, _, _, _, err := bs.normalizeImagePlacement(mediaID, x, y, width, height, rotation, author); err != nil {
		return BoardImage{}, err
	}
	assetID, filename, err := bs.storeBoardAsset(source)
	if err != nil {
		return BoardImage{}, err
	}
	return bs.addImagePlacement(mediaID, assetID, filename, mimeType, x, y, width, height, rotation, flipX, author)
}

// AddExistingImage reuses an existing asset in a new board placement.
func (bs *BoardStore) AddExistingImage(mediaID, assetID string, x, y, width, height, rotation float64, flipX bool, author string) (BoardImage, error) {
	path, mimeType, err := bs.GlobalAssetPath(assetID)
	if err != nil {
		return BoardImage{}, err
	}
	return bs.addImagePlacement(mediaID, assetID, filepath.Base(path), mimeType, x, y, width, height, rotation, flipX, author)
}

func (bs *BoardStore) addImagePlacement(mediaID, assetID, filename, mimeType string, x, y, width, height, rotation float64, flipX bool, author string) (BoardImage, error) {
	state, _, _, _, _, _, _, err := bs.normalizeImagePlacement(mediaID, x, y, width, height, rotation, author)
	if err != nil {
		return BoardImage{}, err
	}
	image, err := bs.newBoardImage(mediaID, assetID, filename, mimeType, x, y, width, height, rotation, flipX, author)
	if err != nil {
		return BoardImage{}, err
	}
	operation := BoardOperation{Type: "image", Image: &image}
	line, err := json.Marshal(operation)
	if err != nil {
		return BoardImage{}, fmt.Errorf("marshal board image: %w", err)
	}
	f, err := os.OpenFile(state.filePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return BoardImage{}, fmt.Errorf("open board file: %w", err)
	}
	if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
		_ = f.Close()
		return BoardImage{}, fmt.Errorf("write board image: %w", err)
	}
	if err := f.Close(); err != nil {
		return BoardImage{}, fmt.Errorf("close board file: %w", err)
	}

	bs.mu.Lock()
	state = bs.boards[mediaID]
	state.operations = append(state.operations, operation)
	bs.mu.Unlock()
	return image, nil
}

func (bs *BoardStore) newBoardImage(mediaID, assetID, filename, mimeType string, x, y, width, height, rotation float64, flipX bool, author string) (BoardImage, error) {
	_, x, y, width, height, rotation, author, err := bs.normalizeImagePlacement(mediaID, x, y, width, height, rotation, author)
	if err != nil {
		return BoardImage{}, err
	}
	return BoardImage{
		ID:        generateStrokeID(),
		AssetID:   assetID,
		URL:       "/api/boards/" + mediaID + "/assets/" + assetID,
		MimeType:  mimeType,
		X:         x,
		Y:         y,
		Width:     width,
		Height:    height,
		Rotation:  rotation,
		FlipX:     flipX,
		Author:    author,
		CreatedAt: time.Now().UTC(),
		Filename:  filename,
	}, nil
}

func (bs *BoardStore) normalizeImagePlacement(mediaID string, x, y, width, height, rotation float64, author string) (*boardState, float64, float64, float64, float64, float64, string, error) {
	if width <= 0 || height <= 0 || math.IsNaN(width) || math.IsInf(width, 0) || math.IsNaN(height) || math.IsInf(height, 0) {
		return nil, 0, 0, 0, 0, 0, "", errors.New("image dimensions must be finite positive numbers")
	}
	values := []*float64{&x, &y, &width, &height, &rotation}
	for _, value := range values {
		normalized, err := normalizeBoardCoordinate(*value)
		if err != nil {
			return nil, 0, 0, 0, 0, 0, "", err
		}
		*value = normalized
	}
	author = strings.TrimSpace(author)
	if author == "" {
		author = "Guest"
	}

	bs.mu.RLock()
	state, ok := bs.boards[mediaID]
	bs.mu.RUnlock()
	if !ok {
		return nil, 0, 0, 0, 0, 0, "", ErrBoardNotFound
	}
	maxWidth := float64(state.info.Canvas.Width) * 10
	maxHeight := float64(state.info.Canvas.Height) * 10
	if width > maxWidth || height > maxHeight ||
		x < -maxWidth || x > maxWidth || y < -maxHeight || y > maxHeight {
		return nil, 0, 0, 0, 0, 0, "", errors.New("image placement is outside allowed board bounds")
	}
	return state, x, y, width, height, rotation, author, nil
}

func (bs *BoardStore) storeBoardAsset(source io.Reader) (string, string, error) {
	assetsDir := filepath.Join(bs.root, boardAssetsDirName)
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create board assets directory: %w", err)
	}

	temp, err := os.CreateTemp(assetsDir, ".asset-*")
	if err != nil {
		return "", "", fmt.Errorf("create temporary board image asset: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(temp, hash), source); err != nil {
		_ = temp.Close()
		return "", "", fmt.Errorf("store board image asset: %w", err)
	}
	if err := temp.Close(); err != nil {
		return "", "", fmt.Errorf("close board image asset: %w", err)
	}

	assetID := hex.EncodeToString(hash.Sum(nil))
	filename := assetID
	assetPath := filepath.Join(assetsDir, filename)
	if err := os.Link(tempPath, assetPath); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return "", "", fmt.Errorf("publish board image asset: %w", err)
		}
	}
	return assetID, filename, nil
}

func (bs *BoardStore) importStickerPack() error {
	stickerPackDir := filepath.Join(bs.root, boardStickerPackDirName)
	entries, err := os.ReadDir(stickerPackDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read sticker pack directory: %w", err)
	}

	for _, entry := range entries {
		if err := bs.importStickerPackEntry(stickerPackDir, entry); err != nil {
			return err
		}
	}
	return nil
}

func (bs *BoardStore) importStickerPackEntry(parent string, entry os.DirEntry) error {
	path := filepath.Join(parent, entry.Name())
	if entry.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return fmt.Errorf("read sticker pack directory %q: %w", path, err)
		}
		for _, child := range entries {
			if err := bs.importStickerPackEntry(path, child); err != nil {
				return err
			}
		}
		return nil
	}
	if !entry.Type().IsRegular() {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open sticker pack image %q: %w", path, err)
	}
	defer file.Close()

	header := make([]byte, 512)
	n, err := io.ReadFull(file, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("read sticker pack image %q: %w", path, err)
	}
	mimeType := http.DetectContentType(header[:n])
	if !isBoardImageMimeType(mimeType) {
		return nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind sticker pack image %q: %w", path, err)
	}
	assetID, filename, err := bs.storeBoardAsset(file)
	if err != nil {
		return fmt.Errorf("store sticker pack image %q: %w", path, err)
	}
	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("stat sticker pack image %q: %w", path, err)
	}
	createdAt := info.ModTime().UTC()
	asset, ok := bs.assets[assetID]
	if !ok || createdAt.After(asset.CreatedAt) {
		bs.assets[assetID] = BoardAsset{
			ID:        assetID,
			URL:       "/api/board-assets/" + assetID,
			MimeType:  mimeType,
			CreatedAt: createdAt,
			Filename:  filename,
		}
	}
	return nil
}

func isBoardImageMimeType(mimeType string) bool {
	switch mimeType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	default:
		return false
	}
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

func (bs *BoardStore) boardFilePath(filename string) string {
	return filepath.Join(bs.root, filename+".jsonl")
}

func (bs *BoardStore) loadBoardFile(mediaID string, filename string) (*boardState, error) {
	filePath := bs.boardFilePath(filename)
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
		meta.Background.URL = "/api/boards/" + mediaID + "/background"
	}
	if meta.Background.Type == "image" && meta.Background.Filename != "" && filepath.Base(meta.Background.Filename) == meta.Background.Filename {
		canvas := imageCanvas(filepath.Join(bs.contentDir, meta.Background.Filename))
		if canvas.Width > 0 && canvas.Height > 0 {
			meta.Canvas = canvas
		}
	}

	var strokes []Stroke
	var operations []BoardOperation
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var operation BoardOperation
		if err := json.Unmarshal(line, &operation); err == nil && operation.Type != "" {
			switch {
			case operation.Type == "stroke" && operation.Stroke != nil:
				normalizeLoadedStroke(operation.Stroke)
				strokes = append(strokes, *operation.Stroke)
				operations = append(operations, operation)
			case operation.Type == "image" && operation.Image != nil && filepath.Base(operation.Image.Filename) == operation.Image.Filename:
				operation.Image.URL = "/api/boards/" + mediaID + "/assets/" + operation.Image.AssetID
				operations = append(operations, operation)
			}
			continue
		}
		var stroke Stroke
		if err := json.Unmarshal(line, &stroke); err != nil || stroke.ID == "" {
			continue
		}
		normalizeLoadedStroke(&stroke)
		strokes = append(strokes, stroke)
		operations = append(operations, BoardOperation{Type: "stroke", Stroke: &stroke})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan board file: %w", err)
	}

	return &boardState{
		filePath: filePath,
		info: BoardInfo{
			ID:          mediaID,
			MediaID:     boardMediaID(mediaID),
			Filename:    filename,
			Name:        meta.Name,
			Background:  meta.Background,
			Canvas:      meta.Canvas,
			StrokeCount: len(strokes),
			CreatedAt:   meta.CreatedAt,
		},
		strokes:    strokes,
		operations: operations,
	}, nil
}

func normalizeLoadedStroke(stroke *Stroke) {
	if stroke.Opacity <= 0 || stroke.Opacity > 1 || math.IsNaN(stroke.Opacity) || math.IsInf(stroke.Opacity, 0) {
		stroke.Opacity = 1
	}
}

func (bs *BoardStore) createBoardFileForPlaceholder(mediaID string, filename string, createdAt time.Time) (*boardState, error) {
	name := defaultBoardName(strings.TrimSuffix(filename, ".board"))
	meta := boardMeta{Name: name, CreatedAt: createdAt, Background: defaultBoardBackground(), Canvas: defaultBoardCanvas()}
	metaLine, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("marshal board meta: %w", err)
	}
	filePath := bs.boardFilePath(filename)
	if err := os.WriteFile(filePath, append(metaLine, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("create board file: %w", err)
	}
	return &boardState{
		filePath: filePath,
		info: BoardInfo{
			ID:          mediaID,
			MediaID:     boardMediaID(mediaID),
			Filename:    filename,
			Name:        name,
			Background:  defaultBoardBackground(),
			Canvas:      defaultBoardCanvas(),
			StrokeCount: 0,
			CreatedAt:   createdAt,
		},
	}, nil
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

func boardMediaID(mediaID string) string {
	if mediaID == "master" {
		return ""
	}
	return mediaID
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
		return normalizeImageBoardCanvas(config.Height, config.Width)
	}
	return normalizeImageBoardCanvas(config.Width, config.Height)
}

func normalizeImageBoardCanvas(width, height int) BoardCanvas {
	if width <= 0 || height <= 0 {
		return defaultBoardCanvas()
	}
	defaultArea := float64(defaultBoardCanvasWidth * defaultBoardCanvasHeight)
	imageArea := float64(width * height)
	if imageArea <= defaultArea {
		return BoardCanvas{Width: width, Height: height}
	}
	scale := math.Sqrt(defaultArea / imageArea)
	normalizedWidth := int(math.Round(float64(width) * scale))
	normalizedHeight := int(math.Round(float64(height) * scale))
	if normalizedWidth < 1 {
		normalizedWidth = 1
	}
	if normalizedHeight < 1 {
		normalizedHeight = 1
	}
	return BoardCanvas{Width: normalizedWidth, Height: normalizedHeight}
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
