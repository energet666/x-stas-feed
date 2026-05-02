package media

import (
	"encoding/base64"
	"errors"
	"io/fs"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultLimit = 8
	maxLimit     = 50
	cacheTTL     = 2 * time.Second
)

var supportedExtensions = map[string]string{
	".avif": "image",
	".gif":  "image",
	".jpeg": "image",
	".jpg":  "image",
	".png":  "image",
	".webp": "image",
	".m4v":  "video",
	".mov":  "video",
	".mp4":  "video",
	".ogg":  "video",
	".ogv":  "video",
	".webm": "video",
}

type Library struct {
	root       string
	comments   *CommentStore
	mu         sync.Mutex
	cachedAt   time.Time
	cachedList []Item
}

type Item struct {
	ID           string    `json:"id"`
	Filename     string    `json:"filename"`
	Type         string    `json:"type"`
	URL          string    `json:"url"`
	MimeType     string    `json:"mimeType"`
	Size         int64     `json:"size"`
	ModifiedAt   time.Time `json:"modifiedAt"`
	Comments     []Comment `json:"comments"`
	CommentCount int       `json:"commentCount"`
}

type Page struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}

func NewLibrary(root string) *Library {
	return &Library{root: root, comments: NewCommentStore(root)}
}

func (l *Library) Page(cursor string, requestedLimit int) (Page, error) {
	items, err := l.cachedScan()
	if err != nil {
		return Page{}, err
	}

	start, err := parseCursor(cursor)
	if err != nil {
		return Page{}, err
	}
	if start > len(items) {
		start = len(items)
	}

	limit := normalizeLimit(requestedLimit)
	end := start + limit
	if end > len(items) {
		end = len(items)
	}

	pageItems := append([]Item(nil), items[start:end]...)
	for i := range pageItems {
		comments, count, err := l.comments.Summary(pageItems[i].ID, 2)
		if err != nil {
			return Page{}, err
		}
		pageItems[i].Comments = comments
		pageItems[i].CommentCount = count
	}

	page := Page{Items: pageItems}
	if end < len(items) {
		page.NextCursor = strconv.Itoa(end)
	}

	return page, nil
}

func (l *Library) CommentsForID(id string) ([]Comment, error) {
	if _, _, err := l.PathForID(id); err != nil {
		return nil, err
	}
	return l.comments.List(id)
}

func (l *Library) AddComment(id, text string) (Comment, error) {
	if _, _, err := l.PathForID(id); err != nil {
		return Comment{}, err
	}
	return l.comments.Add(id, text)
}

func (l *Library) Scan() ([]Item, error) {
	root, err := filepath.Abs(l.root)
	if err != nil {
		return nil, err
	}

	var items []Item
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		kind, ok := kindForPath(path)
		if !ok {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		items = append(items, Item{
			ID:         EncodeID(rel),
			Filename:   filepath.Base(path),
			Type:       kind,
			URL:        "/media/" + url.PathEscape(EncodeID(rel)),
			MimeType:   mimeType(path),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().UTC(),
		})

		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return []Item{}, nil
	}
	if err != nil {
		return nil, err
	}

	sort.Slice(items, func(i, j int) bool {
		if !items[i].ModifiedAt.Equal(items[j].ModifiedAt) {
			return items[i].ModifiedAt.After(items[j].ModifiedAt)
		}
		return items[i].Filename < items[j].Filename
	})

	return items, nil
}

func (l *Library) cachedScan() ([]Item, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if time.Since(l.cachedAt) < cacheTTL {
		return append([]Item(nil), l.cachedList...), nil
	}

	items, err := l.Scan()
	if err != nil {
		return nil, err
	}

	l.cachedAt = time.Now()
	l.cachedList = append([]Item(nil), items...)
	return items, nil
}

func (l *Library) PathForID(id string) (string, string, error) {
	rel, err := DecodeID(id)
	if err != nil {
		return "", "", err
	}
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(filepath.ToSlash(rel), "../") || strings.HasPrefix(filepath.ToSlash(rel), "..") {
		return "", "", errors.New("invalid media id")
	}

	path := filepath.Join(l.root, filepath.FromSlash(rel))
	root, err := filepath.Abs(l.root)
	if err != nil {
		return "", "", err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	if absPath != root && !strings.HasPrefix(absPath, root+string(os.PathSeparator)) {
		return "", "", errors.New("media path escapes root")
	}

	if _, ok := kindForPath(absPath); !ok {
		return "", "", errors.New("unsupported media type")
	}
	if _, err := os.Stat(absPath); err != nil {
		return "", "", err
	}

	return absPath, mimeType(absPath), nil
}

func EncodeID(rel string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(filepath.ToSlash(rel)))
}

func DecodeID(id string) (string, error) {
	bytes, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(string(bytes)), nil
}

func parseCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	start, err := strconv.Atoi(cursor)
	if err != nil || start < 0 {
		return 0, errors.New("invalid cursor")
	}
	return start, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func kindForPath(path string) (string, bool) {
	kind, ok := supportedExtensions[strings.ToLower(filepath.Ext(path))]
	return kind, ok
}

func mimeType(path string) string {
	if typ := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); typ != "" {
		return typ
	}
	if kind, ok := kindForPath(path); ok && kind == "video" {
		return "video/mp4"
	}
	return "application/octet-stream"
}
