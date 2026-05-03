package media

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	commentsDirName = ".comments"
	maxCommentBytes = 2000
	maxAuthorRunes  = 40
	defaultAuthor   = "Guest"
)

type CommentStore struct {
	root string
	mu   sync.Mutex
}

type Comment struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewCommentStore(mediaRoot string) *CommentStore {
	return &CommentStore{root: filepath.Join(mediaRoot, commentsDirName)}
}

func (s *CommentStore) List(mediaID string) ([]Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.readLocked(mediaID)
}

func (s *CommentStore) Add(mediaID, text, author string) (Comment, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Comment{}, errors.New("comment text is required")
	}
	if len([]byte(text)) > maxCommentBytes {
		return Comment{}, errors.New("comment text is too long")
	}

	comment := Comment{
		ID:        newCommentID(),
		Author:    normalizeAuthor(author),
		Text:      text,
		CreatedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return Comment{}, err
	}

	file, err := os.OpenFile(s.pathForID(mediaID), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return Comment{}, err
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(comment); err != nil {
		return Comment{}, err
	}

	return comment, nil
}

func (s *CommentStore) Summary(mediaID string, limit int) ([]Comment, int, error) {
	comments, err := s.List(mediaID)
	if err != nil {
		return nil, 0, err
	}
	if limit <= 0 || len(comments) <= limit {
		return comments, len(comments), nil
	}
	return comments[len(comments)-limit:], len(comments), nil
}

func (s *CommentStore) readLocked(mediaID string) ([]Comment, error) {
	file, err := os.Open(s.pathForID(mediaID))
	if errors.Is(err, os.ErrNotExist) {
		return []Comment{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var comments []Comment
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), maxCommentBytes+maxAuthorRunes*4+2048)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var comment Comment
		if err := json.Unmarshal([]byte(line), &comment); err != nil {
			return nil, err
		}
		comment.Author = normalizeAuthor(comment.Author)
		comments = append(comments, comment)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return comments, nil
}

func normalizeAuthor(author string) string {
	author = strings.Join(strings.Fields(author), " ")
	if author == "" {
		return defaultAuthor
	}

	runes := []rune(author)
	if len(runes) > maxAuthorRunes {
		return string(runes[:maxAuthorRunes])
	}
	return author
}

func (s *CommentStore) pathForID(mediaID string) string {
	return filepath.Join(s.root, mediaID+".jsonl")
}

func newCommentID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return time.Now().UTC().Format("20060102T150405.000000000")
	}
	return time.Now().UTC().Format("20060102T150405.000000000") + "-" + hex.EncodeToString(bytes[:])
}
