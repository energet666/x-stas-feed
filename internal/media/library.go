package media

import (
	"encoding/base64"
	"errors"
	"io/fs"
	"log"
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
	defaultLimit         = 8
	maxLimit             = 50
	defaultActivityLimit = 30
	maxActivityLimit     = 100
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

func SupportedExtensions() []string {
	extensions := make([]string, 0, len(supportedExtensions))
	for extension := range supportedExtensions {
		extensions = append(extensions, extension)
	}
	sort.Strings(extensions)
	return extensions
}

type Library struct {
	root     string
	comments *CommentStore
	metadata *MetadataStore
	logger   *log.Logger

	mu                sync.RWMutex
	initialized       bool
	items             []Item
	itemsByID         map[string]Item
	pathsByID         map[string]string
	mimeTypesByID     map[string]string
	commentsByMediaID map[string][]Comment
	activity          []ActivityItem
}

type Item struct {
	ID           string    `json:"id"`
	Filename     string    `json:"filename"`
	DisplayName  string    `json:"displayName"`
	Type         string    `json:"type"`
	URL          string    `json:"url"`
	MimeType     string    `json:"mimeType"`
	Size         int64     `json:"size"`
	ModifiedAt   time.Time `json:"modifiedAt"`
	Comments     []Comment `json:"comments"`
	CommentCount int       `json:"commentCount"`
	LikeCount    int       `json:"likeCount"`
}

type Page struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type ActivityItem struct {
	MediaID          string  `json:"mediaId"`
	MediaDisplayName string  `json:"mediaDisplayName"`
	MediaType        string  `json:"mediaType"`
	Comment          Comment `json:"comment"`
}

func NewLibrary(root string) *Library {
	return &Library{root: root, comments: NewCommentStore(root), metadata: NewMetadataStore(root)}
}

func NewLibraryWithLogger(root string, logger *log.Logger) *Library {
	library := NewLibrary(root)
	library.logger = logger
	return library
}

func (l *Library) Page(cursor string, requestedLimit int) (Page, error) {
	start, err := parseCursor(cursor)
	if err != nil {
		return Page{}, err
	}

	if err := l.ensureIndex(); err != nil {
		return Page{}, err
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	items := l.items
	if start > len(items) {
		start = len(items)
	}

	limit := normalizeLimit(requestedLimit)
	end := start + limit
	if end > len(items) {
		end = len(items)
	}

	pageItems := cloneItems(items[start:end])

	page := Page{Items: pageItems}
	if end < len(items) {
		page.NextCursor = strconv.Itoa(end)
	}

	return page, nil
}

func (l *Library) FavoritePage(ids []string, cursor string, requestedLimit int) (Page, error) {
	start, err := parseCursor(cursor)
	if err != nil {
		return Page{}, err
	}

	if err := l.ensureIndex(); err != nil {
		return Page{}, err
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	if start > len(ids) {
		start = len(ids)
	}

	limit := normalizeLimit(requestedLimit)
	pageItems := make([]Item, 0, limit)
	end := start
	for end < len(ids) && len(pageItems) < limit {
		if item, ok := l.itemsByID[ids[end]]; ok {
			pageItems = append(pageItems, cloneItem(item))
		}
		end++
	}

	page := Page{Items: pageItems}
	if end < len(ids) {
		page.NextCursor = strconv.Itoa(end)
	}

	return page, nil
}

func (l *Library) ItemForID(id string) (Item, error) {
	if err := validateMediaID(id); err != nil {
		return Item{}, err
	}
	if err := l.ensureIndex(); err != nil {
		return Item{}, err
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	item, ok := l.itemsByID[id]
	if !ok {
		return Item{}, os.ErrNotExist
	}

	return cloneItem(item), nil
}

func (l *Library) Activity(requestedLimit int) ([]ActivityItem, error) {
	if err := l.ensureIndex(); err != nil {
		return nil, err
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	limit := normalizeActivityLimit(requestedLimit)
	if len(l.activity) < limit {
		limit = len(l.activity)
	}
	activity := make([]ActivityItem, limit)
	copy(activity, l.activity[:limit])
	return activity, nil
}

func (l *Library) CommentsForID(id string) ([]Comment, error) {
	if err := validateMediaID(id); err != nil {
		return nil, err
	}
	if err := l.ensureIndex(); err != nil {
		return nil, err
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	if _, ok := l.itemsByID[id]; !ok {
		return nil, os.ErrNotExist
	}
	return cloneComments(l.commentsByMediaID[id]), nil
}

func (l *Library) AddComment(id, text, author string) (Comment, error) {
	started := time.Now()
	if err := validateMediaID(id); err != nil {
		return Comment{}, err
	}
	if err := l.ensureIndex(); err != nil {
		return Comment{}, err
	}

	l.mu.RLock()
	_, ok := l.itemsByID[id]
	l.mu.RUnlock()
	if !ok {
		return Comment{}, os.ErrNotExist
	}

	comment, err := l.comments.Add(id, text, author)
	if err != nil {
		return Comment{}, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	item, ok := l.itemsByID[id]
	if !ok {
		return Comment{}, os.ErrNotExist
	}
	l.commentsByMediaID[id] = append(l.commentsByMediaID[id], comment)
	l.updateItemCommentSummaryLocked(id)
	l.insertActivityLocked(item, comment)
	l.logf("comment appended mediaID=%s filename=%s commentID=%s duration=%s totalCommentsForMedia=%d activityItems=%d", id, item.Filename, comment.ID, time.Since(started).Round(time.Millisecond), len(l.commentsByMediaID[id]), len(l.activity))
	return comment, nil
}

func (l *Library) AddCommentLike(id, commentID string) (Comment, error) {
	started := time.Now()
	if err := validateMediaID(id); err != nil {
		return Comment{}, err
	}
	if err := l.ensureIndex(); err != nil {
		return Comment{}, err
	}

	l.mu.RLock()
	_, ok := l.itemsByID[id]
	l.mu.RUnlock()
	if !ok {
		return Comment{}, os.ErrNotExist
	}

	comment, err := l.comments.AddLike(id, commentID)
	if err != nil {
		return Comment{}, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	comments := l.commentsByMediaID[id]
	for i := range comments {
		if comments[i].ID == comment.ID {
			comments[i] = comment
			l.commentsByMediaID[id] = comments
			l.updateItemCommentSummaryLocked(id)
			l.updateActivityCommentLocked(id, comment)
			item := l.itemsByID[id]
			l.logf("comment like persisted mediaID=%s filename=%s commentID=%s likeCount=%d duration=%s commentsForMedia=%d", id, item.Filename, comment.ID, comment.LikeCount, time.Since(started).Round(time.Millisecond), len(comments))
			return comment, nil
		}
	}

	return Comment{}, ErrCommentNotFound
}

func (l *Library) AddLike(id string) (int, error) {
	started := time.Now()
	if err := validateMediaID(id); err != nil {
		return 0, err
	}
	if err := l.ensureIndex(); err != nil {
		return 0, err
	}

	l.mu.RLock()
	_, ok := l.itemsByID[id]
	l.mu.RUnlock()
	if !ok {
		return 0, os.ErrNotExist
	}

	metadata, err := l.metadata.AddLike(id)
	if err != nil {
		return 0, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	item, ok := l.itemsByID[id]
	if !ok {
		return 0, os.ErrNotExist
	}
	item.LikeCount = metadata.LikeCount
	l.setItemLocked(item)
	l.logf("media like persisted mediaID=%s filename=%s likeCount=%d duration=%s", id, item.Filename, metadata.LikeCount, time.Since(started).Round(time.Millisecond))
	return metadata.LikeCount, nil
}

func (l *Library) Scan() ([]Item, error) {
	started := time.Now()
	l.logf("media scan started root=%s", l.root)

	root, err := filepath.Abs(l.root)
	if err != nil {
		return nil, err
	}

	var items []Item
	var scannedFiles int
	var supportedFiles int
	var unsupportedFiles int
	var skippedDirs int
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (entry.Name() == commentsDirName || entry.Name() == posterDirName || entry.Name() == metadataDirName) {
				skippedDirs++
				return filepath.SkipDir
			}
			return nil
		}

		scannedFiles++
		kind, ok := kindForPath(path)
		if !ok {
			unsupportedFiles++
			return nil
		}
		supportedFiles++

		info, err := entry.Info()
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		item := itemFromFile(rel, path, kind, info)
		if metadata, err := l.metadata.Get(item.ID); err == nil {
			if metadata.DisplayName != "" {
				item.DisplayName = metadata.DisplayName
			}
			item.LikeCount = metadata.LikeCount
		}
		items = append(items, item)

		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		l.logf("media scan completed root=%s duration=%s scannedFiles=0 supportedMedia=0 unsupportedFiles=0 skippedInternalDirs=0 items=0 missingRoot=true", root, time.Since(started).Round(time.Millisecond))
		return []Item{}, nil
	}
	if err != nil {
		l.logf("media scan failed root=%s duration=%s scannedFiles=%d supportedMedia=%d unsupportedFiles=%d skippedInternalDirs=%d error=%v", root, time.Since(started).Round(time.Millisecond), scannedFiles, supportedFiles, unsupportedFiles, skippedDirs, err)
		return nil, err
	}

	sortItems(items)
	l.logf("media scan completed root=%s duration=%s scannedFiles=%d supportedMedia=%d unsupportedFiles=%d skippedInternalDirs=%d items=%d", root, time.Since(started).Round(time.Millisecond), scannedFiles, supportedFiles, unsupportedFiles, skippedDirs, len(items))

	return items, nil
}

func (l *Library) ensureIndex() error {
	l.mu.RLock()
	if l.initialized {
		l.mu.RUnlock()
		return nil
	}
	l.mu.RUnlock()

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.initialized {
		return nil
	}

	started := time.Now()
	l.logf("runtime index initialization started root=%s", l.root)

	items, err := l.Scan()
	if err != nil {
		l.logf("runtime index initialization failed root=%s duration=%s error=%v", l.root, time.Since(started).Round(time.Millisecond), err)
		return err
	}

	l.items = make([]Item, len(items))
	l.itemsByID = make(map[string]Item, len(items))
	l.pathsByID = make(map[string]string, len(items))
	l.mimeTypesByID = make(map[string]string, len(items))
	l.commentsByMediaID = make(map[string][]Comment, len(items))
	l.activity = nil

	root, err := filepath.Abs(l.root)
	if err != nil {
		l.logf("runtime index initialization failed root=%s duration=%s error=%v", l.root, time.Since(started).Round(time.Millisecond), err)
		return err
	}

	var commentFilesFound int
	var commentsFailed int
	var mediaWithoutCommentFile int
	var commentCount int
	for i, item := range items {
		rel, err := DecodeID(item.ID)
		if err != nil {
			l.logf("runtime index initialization failed root=%s duration=%s error=%v", l.root, time.Since(started).Round(time.Millisecond), err)
			return err
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		commentFileExists, commentFileStatErr := l.comments.hasFile(item.ID)
		if commentFileStatErr == nil && commentFileExists {
			commentFilesFound++
		} else if commentFileStatErr == nil {
			mediaWithoutCommentFile++
		} else {
			commentsFailed++
			l.logf("comment cache stat failed mediaID=%s filename=%s error=%v", item.ID, item.Filename, commentFileStatErr)
		}
		comments, err := l.comments.List(item.ID)
		if err == nil {
			commentCount += len(comments)
			l.commentsByMediaID[item.ID] = cloneComments(comments)
			item = itemWithCommentSummary(item, comments)
		} else {
			commentsFailed++
			l.logf("comment cache load failed mediaID=%s filename=%s commentFileExists=%t error=%v", item.ID, item.Filename, commentFileExists, err)
			l.commentsByMediaID[item.ID] = []Comment{}
			item = itemWithCommentSummary(item, nil)
		}
		l.items[i] = item
		l.itemsByID[item.ID] = item
		l.pathsByID[item.ID] = path
		l.mimeTypesByID[item.ID] = item.MimeType
		for _, comment := range l.commentsByMediaID[item.ID] {
			l.activity = append(l.activity, activityItem(item, comment))
		}
	}
	sortActivity(l.activity)
	l.initialized = true
	l.logf("runtime index initialization completed root=%s duration=%s mediaItems=%d commentFilesFound=%d mediaWithoutCommentFile=%d commentFilesFailed=%d comments=%d activityItems=%d", root, time.Since(started).Round(time.Millisecond), len(l.items), commentFilesFound, mediaWithoutCommentFile, commentsFailed, commentCount, len(l.activity))
	return nil
}

func (l *Library) PathForID(id string) (string, string, error) {
	if err := validateMediaID(id); err != nil {
		return "", "", err
	}
	if err := l.ensureIndex(); err != nil {
		return "", "", err
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	path, ok := l.pathsByID[id]
	if !ok {
		return "", "", os.ErrNotExist
	}

	return path, l.mimeTypesByID[id], nil
}

func (l *Library) insertItem(item Item, path string) error {
	if err := l.ensureIndex(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	item = itemWithCommentSummary(item, nil)
	l.items = append(l.items, item)
	sortItems(l.items)
	l.itemsByID[item.ID] = item
	l.pathsByID[item.ID] = path
	l.mimeTypesByID[item.ID] = item.MimeType
	l.commentsByMediaID[item.ID] = []Comment{}
	l.logf("runtime index media inserted mediaID=%s filename=%s type=%s size=%d totalMediaItems=%d", item.ID, item.Filename, item.Type, item.Size, len(l.items))
	return nil
}

func (l *Library) setItemLocked(item Item) {
	l.itemsByID[item.ID] = item
	for i := range l.items {
		if l.items[i].ID == item.ID {
			l.items[i] = item
			return
		}
	}
	l.items = append(l.items, item)
	sortItems(l.items)
}

func (l *Library) updateItemCommentSummaryLocked(id string) {
	item, ok := l.itemsByID[id]
	if !ok {
		return
	}
	l.setItemLocked(itemWithCommentSummary(item, l.commentsByMediaID[id]))
}

func (l *Library) insertActivityLocked(item Item, comment Comment) {
	l.activity = append(l.activity, activityItem(item, comment))
	sortActivity(l.activity)
}

func (l *Library) updateActivityCommentLocked(mediaID string, comment Comment) {
	for i := range l.activity {
		if l.activity[i].MediaID == mediaID && l.activity[i].Comment.ID == comment.ID {
			l.activity[i].Comment = comment
			sortActivity(l.activity)
			return
		}
	}
	if item, ok := l.itemsByID[mediaID]; ok {
		l.insertActivityLocked(item, comment)
	}
}

func itemWithCommentSummary(item Item, comments []Comment) Item {
	item.Comments = latestComments(comments, 2)
	item.CommentCount = len(comments)
	return item
}

func latestComments(comments []Comment, limit int) []Comment {
	if len(comments) == 0 {
		return []Comment{}
	}
	if limit <= 0 || len(comments) <= limit {
		return append([]Comment(nil), comments...)
	}
	return append([]Comment(nil), comments[len(comments)-limit:]...)
}

func cloneItems(items []Item) []Item {
	cloned := make([]Item, len(items))
	for i, item := range items {
		cloned[i] = cloneItem(item)
	}
	return cloned
}

func cloneItem(item Item) Item {
	item.Comments = cloneComments(item.Comments)
	return item
}

func cloneComments(comments []Comment) []Comment {
	if len(comments) == 0 {
		return []Comment{}
	}
	return append([]Comment(nil), comments...)
}

func activityItem(item Item, comment Comment) ActivityItem {
	return ActivityItem{
		MediaID:          item.ID,
		MediaDisplayName: item.DisplayName,
		MediaType:        item.Type,
		Comment:          comment,
	}
}

func sortItems(items []Item) {
	sort.Slice(items, func(i, j int) bool {
		if !items[i].ModifiedAt.Equal(items[j].ModifiedAt) {
			return items[i].ModifiedAt.After(items[j].ModifiedAt)
		}
		return items[i].Filename < items[j].Filename
	})
}

func sortActivity(activity []ActivityItem) {
	sort.Slice(activity, func(i, j int) bool {
		if !activity[i].Comment.CreatedAt.Equal(activity[j].Comment.CreatedAt) {
			return activity[i].Comment.CreatedAt.After(activity[j].Comment.CreatedAt)
		}
		if activity[i].MediaDisplayName != activity[j].MediaDisplayName {
			return activity[i].MediaDisplayName < activity[j].MediaDisplayName
		}
		return activity[i].Comment.ID > activity[j].Comment.ID
	})
}

func validateMediaID(id string) error {
	rel, err := DecodeID(id)
	if err != nil {
		return err
	}
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(filepath.ToSlash(rel), "../") || strings.HasPrefix(filepath.ToSlash(rel), "..") {
		return errors.New("invalid media id")
	}
	if _, ok := kindForPath(rel); !ok {
		return errors.New("unsupported media type")
	}
	return nil
}

func (l *Library) logf(format string, args ...any) {
	if l.logger == nil {
		return
	}
	l.logger.Printf(format, args...)
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

func normalizeActivityLimit(limit int) int {
	if limit <= 0 {
		return defaultActivityLimit
	}
	if limit > maxActivityLimit {
		return maxActivityLimit
	}
	return limit
}

func kindForPath(path string) (string, bool) {
	kind, ok := supportedExtensions[strings.ToLower(filepath.Ext(path))]
	return kind, ok
}

func itemFromFile(rel, path, kind string, info os.FileInfo) Item {
	id := EncodeID(rel)
	return Item{
		ID:          id,
		Filename:    filepath.Base(path),
		DisplayName: filepath.Base(path),
		Type:        kind,
		URL:         "/media/" + url.PathEscape(id),
		MimeType:    mimeType(path),
		Size:        info.Size(),
		ModifiedAt:  info.ModTime().UTC(),
		Comments:    []Comment{},
	}
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
