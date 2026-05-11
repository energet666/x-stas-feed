package media

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const boardsDirName = ".boards"

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
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	StrokeCount int       `json:"strokeCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

// boardMeta is persisted as the first line of the board JSONL file.
type boardMeta struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
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

	entries, err := os.ReadDir(bs.root)
	if err != nil {
		return fmt.Errorf("read boards directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		boardID := strings.TrimSuffix(entry.Name(), ".jsonl")
		state, loadErr := bs.loadBoardFile(boardID)
		if loadErr != nil {
			continue
		}
		bs.boards[boardID] = state
		_ = bs.ensureBoardPlaceholder(boardID)
	}

	// Ensure master board exists
	if _, ok := bs.boards["master"]; !ok {
		now := time.Now().UTC()
		name := "Master Board"
		meta := boardMeta{Name: name, CreatedAt: now}
		metaLine, _ := json.Marshal(meta)
		
		filePath := bs.boardFilePath("master")
		if err := os.WriteFile(filePath, append(metaLine, '\n'), 0o644); err == nil {
			bs.boards["master"] = &boardState{
				info: BoardInfo{
					ID:          "master",
					Name:        name,
					StrokeCount: 0,
					CreatedAt:   now,
				},
			}
			_ = bs.ensureBoardPlaceholder("master")
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
		name = "Board"
	}

	meta := boardMeta{Name: name, CreatedAt: now}
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
		Name:        name,
		StrokeCount: 0,
		CreatedAt:   now,
	}

	bs.mu.Lock()
	bs.boards[id] = &boardState{info: info, strokes: nil}
	bs.mu.Unlock()

	_ = bs.ensureBoardPlaceholder(id)

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

// AddStroke appends a stroke to the board file and updates the in-memory state.
func (bs *BoardStore) AddStroke(id string, tool string, points [][]float64, color string, size float64, author string) (Stroke, error) {
	tool = strings.TrimSpace(tool)
	if tool != "freeform" && tool != "line" {
		return Stroke{}, errors.New("invalid tool: must be freeform or line")
	}
	if len(points) < 2 {
		return Stroke{}, errors.New("stroke must have at least 2 points")
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
		Points:    points,
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
			Name:        meta.Name,
			StrokeCount: len(strokes),
			CreatedAt:   meta.CreatedAt,
		},
		strokes: strokes,
	}, nil
}

func generateBoardID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("generate board id: %v", err))
	}
	return hex.EncodeToString(b)
}

func generateStrokeID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("generate stroke id: %v", err))
	}
	return hex.EncodeToString(b)
}

func (bs *BoardStore) ensureBoardPlaceholder(id string) error {
	placeholderPath := filepath.Join(bs.contentDir, id+".board")
	if _, err := os.Stat(placeholderPath); err == nil {
		return nil
	}
	return os.WriteFile(placeholderPath, []byte{}, 0o644)
}
