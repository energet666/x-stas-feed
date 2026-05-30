package media

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"log"
	"mime"
	"net/http"
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

var mediaExtensions = map[string]string{
	".avif":  "image",
	".gif":   "image",
	".jpeg":  "image",
	".jpg":   "image",
	".png":   "image",
	".webp":  "image",
	".aac":   "audio",
	".flac":  "audio",
	".m4a":   "audio",
	".mp3":   "audio",
	".oga":   "audio",
	".opus":  "audio",
	".wav":   "audio",
	".m4v":   "video",
	".mov":   "video",
	".mp4":   "video",
	".ogg":   "video",
	".ogv":   "video",
	".webm":  "video",
	".board": "board",
}

func SupportedExtensions() []string {
	extensions := make([]string, 0, len(mediaExtensions))
	for extension := range mediaExtensions {
		extensions = append(extensions, extension)
	}
	sort.Strings(extensions)
	return extensions
}

type Library struct {
	root     string
	comments *CommentStore
	metadata *MetadataStore
	boards   *BoardStore
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
	ID              string     `json:"id"`
	Filename        string     `json:"filename"`
	DisplayName     string     `json:"displayName"`
	Type            string     `json:"type"`
	URL             string     `json:"url"`
	MimeType        string     `json:"mimeType"`
	Size            int64      `json:"size"`
	ModifiedAt      time.Time  `json:"modifiedAt"`
	Comments        []Comment  `json:"comments"`
	CommentCount    int        `json:"commentCount"`
	LikeCount       int        `json:"likeCount"`
	DurationSeconds float64    `json:"durationSeconds,omitempty"`
	AudioTags       *AudioTags `json:"audioTags,omitempty"`
	CoverURL        string     `json:"coverUrl,omitempty"`
	HasCover        bool       `json:"-"`
	CoverFile       string     `json:"-"`

	sortModifiedAt time.Time
	sourcePath     string
}

type Page struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type IndexedItem struct {
	Index      int  `json:"index"`
	FirstIndex int  `json:"firstIndex"`
	LastIndex  int  `json:"lastIndex"`
	Item       Item `json:"item"`
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

func (l *Library) UseBoardStore(boards *BoardStore) {
	l.boards = boards
}

func (l *Library) IndexedItem(index int) (IndexedItem, error) {
	if err := l.ensureIndex(); err != nil {
		return IndexedItem{}, err
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.items) == 0 {
		return IndexedItem{}, os.ErrNotExist
	}
	if index == -1 {
		index = len(l.items) - 1
	}
	if index < 0 || index >= len(l.items) {
		return IndexedItem{}, os.ErrNotExist
	}

	return IndexedItem{
		Index:      index,
		FirstIndex: 0,
		LastIndex:  len(l.items) - 1,
		Item:       cloneItem(l.items[index]),
	}, nil
}

func (l *Library) IndexedItemForID(id string) (IndexedItem, error) {
	if err := l.ensureIndex(); err != nil {
		return IndexedItem{}, err
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	for index, item := range l.items {
		if item.ID == id {
			return IndexedItem{
				Index:      index,
				FirstIndex: 0,
				LastIndex:  len(l.items) - 1,
				Item:       cloneItem(item),
			}, nil
		}
	}

	return IndexedItem{}, os.ErrNotExist
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
	l.logf("comment appended mediaID=%s filename=%q commentID=%s duration=%s totalCommentsForMedia=%d activityItems=%d", id, item.Filename, comment.ID, time.Since(started).Round(time.Millisecond), len(l.commentsByMediaID[id]), len(l.activity))
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
			l.logf("comment like persisted mediaID=%s filename=%q commentID=%s likeCount=%d duration=%s commentsForMedia=%d", id, item.Filename, comment.ID, comment.LikeCount, time.Since(started).Round(time.Millisecond), len(comments))
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
	l.logf("media like persisted mediaID=%s filename=%q likeCount=%d duration=%s", id, item.Filename, metadata.LikeCount, time.Since(started).Round(time.Millisecond))
	return metadata.LikeCount, nil
}

func (l *Library) Scan() ([]Item, error) {
	started := time.Now()
	l.logf("media scan started root=%q", l.root)

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
			if path != root && strings.HasPrefix(entry.Name(), ".") {
				skippedDirs++
				return filepath.SkipDir
			}
			return nil
		}

		scannedFiles++
		kind, ok := kindForFile(path)
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
		metadataExists, metadataExistsErr := l.metadata.Exists(item.ID)
		if metadataExistsErr != nil {
			l.logf("metadata stat failed mediaID=%s filename=%q error=%v", item.ID, item.Filename, metadataExistsErr)
		}
		metadata, metadataErr := l.metadata.Get(item.ID)
		if metadataErr == nil {
			if metadata.DisplayName != "" {
				item.DisplayName = metadata.DisplayName
			}
			if !metadata.ModifiedAt.IsZero() {
				item.ModifiedAt = metadata.ModifiedAt
			}
			item.LikeCount = metadata.LikeCount
		} else {
			l.logf("metadata load failed mediaID=%s filename=%q error=%v", item.ID, item.Filename, metadataErr)
		}
		metadataChanged := false
		if !metadataExists && metadataExistsErr == nil {
			metadata.DisplayName = item.DisplayName
			metadata.LikeCount = item.LikeCount
			metadataChanged = true
		}
		if updatedMetadata, ok := l.applyAudioMetadata(&item, path, info, metadata); ok {
			metadata.Audio = updatedMetadata.Audio
			metadataChanged = true
		}
		if updatedMetadata, ok := l.applyVideoMetadata(&item, path, info, metadata); ok {
			metadata.Audio = updatedMetadata.Audio
			metadata.Video = updatedMetadata.Video
			metadataChanged = true
		}
		if metadataChanged {
			if err := l.metadata.Set(item.ID, metadata); err != nil {
				l.logf("metadata cache write failed mediaID=%s filename=%q error=%v", item.ID, item.Filename, err)
			}
		}
		items = append(items, item)

		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		l.logf("media scan completed root=%q duration=%s scannedFiles=0 supportedMedia=0 unsupportedFiles=0 skippedInternalDirs=0 items=0 missingRoot=true", root, time.Since(started).Round(time.Millisecond))
		return []Item{}, nil
	}
	if err != nil {
		l.logf("media scan failed root=%q duration=%s scannedFiles=%d supportedMedia=%d unsupportedFiles=%d skippedInternalDirs=%d error=%v", root, time.Since(started).Round(time.Millisecond), scannedFiles, supportedFiles, unsupportedFiles, skippedDirs, err)
		return nil, err
	}

	sortItems(items)
	l.logf("media scan completed root=%q duration=%s scannedFiles=%d supportedMedia=%d unsupportedFiles=%d skippedInternalDirs=%d items=%d", root, time.Since(started).Round(time.Millisecond), scannedFiles, supportedFiles, unsupportedFiles, skippedDirs, len(items))

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
	l.logf("runtime index initialization started root=%q", l.root)

	items, err := l.Scan()
	if err != nil {
		l.logf("runtime index initialization failed root=%q duration=%s error=%v", l.root, time.Since(started).Round(time.Millisecond), err)
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
		l.logf("runtime index initialization failed root=%q duration=%s error=%v", l.root, time.Since(started).Round(time.Millisecond), err)
		return err
	}

	var commentFilesFound int
	var commentsFailed int
	var mediaWithoutCommentFile int
	var commentCount int
	for i, item := range items {
		path := item.sourcePath
		if path == "" {
			l.logf("runtime index initialization failed root=%q duration=%s mediaID=%s filename=%q error=missing source path", l.root, time.Since(started).Round(time.Millisecond), item.ID, item.Filename)
			return errors.New("missing source path for media item")
		}
		commentFileExists, commentFileStatErr := l.comments.hasFile(item.ID)
		if commentFileStatErr == nil && commentFileExists {
			commentFilesFound++
		} else if commentFileStatErr == nil {
			mediaWithoutCommentFile++
		} else {
			commentsFailed++
			l.logf("comment cache stat failed mediaID=%s filename=%q error=%v", item.ID, item.Filename, commentFileStatErr)
		}
		comments, err := l.comments.List(item.ID)
		if err == nil {
			commentCount += len(comments)
			l.commentsByMediaID[item.ID] = cloneComments(comments)
			item = itemWithCommentSummary(item, comments)
		} else {
			commentsFailed++
			l.logf("comment cache load failed mediaID=%s filename=%q commentFileExists=%t error=%v", item.ID, item.Filename, commentFileExists, err)
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
	l.logf("runtime index initialization completed root=%q duration=%s mediaItems=%d commentFilesFound=%d mediaWithoutCommentFile=%d commentFilesFailed=%d comments=%d activityItems=%d", root, time.Since(started).Round(time.Millisecond), len(l.items), commentFilesFound, mediaWithoutCommentFile, commentsFailed, commentCount, len(l.activity))
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
	nextItems := l.items[:0]
	for _, existing := range l.items {
		if existing.ID != item.ID {
			nextItems = append(nextItems, existing)
		}
	}
	l.items = append(nextItems, item)
	sortItems(l.items)
	l.itemsByID[item.ID] = item
	l.pathsByID[item.ID] = path
	l.mimeTypesByID[item.ID] = item.MimeType
	l.commentsByMediaID[item.ID] = []Comment{}
	l.logf("runtime index media inserted mediaID=%s filename=%q type=%s size=%d totalMediaItems=%d", item.ID, item.Filename, item.Type, item.Size, len(l.items))
	return nil
}

func (l *Library) InsertBoardPlaceholder(boardID, displayName string) (Item, error) {
	return l.InsertBoardPlaceholderWithModifiedAt(boardID, displayName, time.Time{})
}

func (l *Library) InsertBoardPlaceholderWithModifiedAt(boardID, displayName string, modifiedAt time.Time) (Item, error) {
	if strings.TrimSpace(boardID) == "" || strings.EqualFold(boardID, "master") {
		return Item{}, errors.New("invalid board id")
	}

	path := filepath.Join(l.root, boardID+".board")
	info, err := os.Stat(path)
	if err != nil {
		return Item{}, err
	}

	rel, err := filepath.Rel(l.root, path)
	if err != nil {
		return Item{}, err
	}
	rel = filepath.ToSlash(rel)

	item := itemFromFile(rel, path, "board", info)
	if name := strings.TrimSpace(displayName); name != "" {
		item.DisplayName = name
	}
	metadata := Metadata{DisplayName: item.DisplayName, LikeCount: item.LikeCount}
	if !modifiedAt.IsZero() {
		metadata.ModifiedAt = modifiedAt.UTC()
		item.ModifiedAt = metadata.ModifiedAt
	}

	if err := l.metadata.Set(item.ID, metadata); err != nil {
		return Item{}, err
	}
	if err := l.insertItem(item, path); err != nil {
		return Item{}, err
	}

	return cloneItem(item), nil
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

func (l *Library) itemSnapshot(id string) Item {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.itemsByID[id]
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
	if item.AudioTags != nil {
		tags := *item.AudioTags
		item.AudioTags = &tags
	}
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
		iTime := sortTime(items[i])
		jTime := sortTime(items[j])
		if !iTime.Equal(jTime) {
			return iTime.Before(jTime)
		}
		return items[i].Filename < items[j].Filename
	})
}

func sortTime(item Item) time.Time {
	if !item.sortModifiedAt.IsZero() {
		return item.sortModifiedAt
	}
	return item.ModifiedAt
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
	if len(id) != sha256.Size*2 {
		return errors.New("invalid media id")
	}
	for _, char := range id {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return errors.New("invalid media id")
		}
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
	normalized := filepath.ToSlash(rel)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
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
	if strings.HasPrefix(filepath.Base(path), ".") {
		return "", false
	}
	if kind, ok := mediaExtensions[strings.ToLower(filepath.Ext(path))]; ok {
		return kind, true
	}
	return "file", true
}

func kindForFile(path string) (string, bool) {
	kind, ok := kindForPath(path)
	if !ok {
		return "", false
	}
	if strings.EqualFold(filepath.Ext(path), ".ogg") {
		if probed, err := probeMedia(path); err == nil && (probed.Kind == "audio" || probed.Kind == "video") {
			return probed.Kind, true
		}
	}
	return kind, true
}

func (l *Library) applyAudioMetadata(item *Item, path string, info os.FileInfo, metadata Metadata) (Metadata, bool) {
	if item.Type != "audio" && !strings.EqualFold(filepath.Ext(path), ".ogg") {
		return metadata, false
	}
	if metadata.Audio != nil && audioMetadataMatches(*metadata.Audio, info) {
		l.applyCachedAudioMetadata(item, *metadata.Audio)
		return metadata, false
	}
	probed, err := probeMedia(path)
	if err != nil {
		l.logf("audio metadata probe failed mediaID=%s filename=%q error=%v", item.ID, item.Filename, err)
		return metadata, false
	}
	if probed.Kind == "video" {
		item.Type = "video"
		metadata.Audio = nil
		video := VideoMetadata{
			DurationSeconds:       probed.DurationSeconds,
			SourceSize:            info.Size(),
			SourceModTimeUnixNano: info.ModTime().UnixNano(),
			ProbedAt:              time.Now().UTC(),
		}
		metadata.Video = &video
		l.applyCachedVideoMetadata(item, video)
		return metadata, true
	}
	if probed.Kind == "audio" {
		item.Type = "audio"
	}
	if item.Type != "audio" {
		return metadata, false
	}

	audio := AudioMetadata{
		DurationSeconds:       probed.DurationSeconds,
		Tags:                  probed.Tags,
		SourceSize:            info.Size(),
		SourceModTimeUnixNano: info.ModTime().UnixNano(),
		ProbedAt:              time.Now().UTC(),
	}
	if probed.HasCover {
		if coverFile, err := l.extractAudioCover(item.ID, path, info.Size(), info.ModTime().UnixNano()); err == nil {
			audio.HasCover = true
			audio.CoverFile = coverFile
		} else {
			l.logf("audio cover extraction failed mediaID=%s filename=%q error=%v", item.ID, item.Filename, err)
		}
	}
	metadata.Audio = &audio
	l.applyCachedAudioMetadata(item, audio)
	return metadata, true
}

func (l *Library) applyVideoMetadata(item *Item, path string, info os.FileInfo, metadata Metadata) (Metadata, bool) {
	if item.Type != "video" {
		return metadata, false
	}
	if metadata.Video != nil && videoMetadataMatches(*metadata.Video, info) {
		l.applyCachedVideoMetadata(item, *metadata.Video)
		return metadata, false
	}
	probed, err := probeMedia(path)
	if err != nil {
		l.logf("video metadata probe failed mediaID=%s filename=%q error=%v", item.ID, item.Filename, err)
		return metadata, false
	}
	if probed.Kind == "audio" {
		item.Type = "audio"
		metadata.Video = nil
		audio := AudioMetadata{
			DurationSeconds:       probed.DurationSeconds,
			Tags:                  probed.Tags,
			SourceSize:            info.Size(),
			SourceModTimeUnixNano: info.ModTime().UnixNano(),
			ProbedAt:              time.Now().UTC(),
		}
		metadata.Audio = &audio
		l.applyCachedAudioMetadata(item, audio)
		return metadata, true
	}
	if probed.Kind != "video" {
		return metadata, false
	}

	video := VideoMetadata{
		DurationSeconds:       probed.DurationSeconds,
		SourceSize:            info.Size(),
		SourceModTimeUnixNano: info.ModTime().UnixNano(),
		ProbedAt:              time.Now().UTC(),
	}
	metadata.Video = &video
	l.applyCachedVideoMetadata(item, video)
	return metadata, true
}

func (l *Library) applyCachedAudioMetadata(item *Item, audio AudioMetadata) {
	item.Type = "audio"
	if audio.DurationSeconds > 0 {
		item.DurationSeconds = audio.DurationSeconds
	}
	if !audio.Tags.empty() {
		tags := audio.Tags
		item.AudioTags = &tags
	}
	item.HasCover = audio.HasCover && audio.CoverFile != ""
	item.CoverFile = audio.CoverFile
	if item.HasCover {
		item.CoverURL = "/api/media/" + url.PathEscape(item.ID) + "/cover"
	}
}

func audioMetadataMatches(audio AudioMetadata, info os.FileInfo) bool {
	return audio.SourceSize == info.Size() && audio.SourceModTimeUnixNano == info.ModTime().UnixNano()
}

func (l *Library) applyCachedVideoMetadata(item *Item, video VideoMetadata) {
	item.Type = "video"
	if video.DurationSeconds > 0 {
		item.DurationSeconds = video.DurationSeconds
	}
}

func videoMetadataMatches(video VideoMetadata, info os.FileInfo) bool {
	return video.SourceSize == info.Size() && video.SourceModTimeUnixNano == info.ModTime().UnixNano()
}

func itemFromFile(rel, path, kind string, info os.FileInfo) Item {
	id := EncodeID(rel)
	displayName := filepath.Base(path)

	if kind == "board" {
		displayName = strings.TrimSuffix(filepath.Base(path), ".board")
		if displayName == "master" {
			displayName = "Master Board"
		}
	}

	return Item{
		ID:             id,
		Filename:       filepath.Base(path),
		DisplayName:    displayName,
		Type:           kind,
		URL:            "/media/" + url.PathEscape(id),
		MimeType:       mimeType(path),
		Size:           info.Size(),
		ModifiedAt:     info.ModTime().UTC(),
		Comments:       []Comment{},
		sortModifiedAt: info.ModTime().UTC(),
		sourcePath:     path,
	}
}

func mimeType(path string) string {
	if typ := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); typ != "" {
		return typ
	}
	if kind, ok := kindForPath(path); ok && kind == "video" {
		return "video/mp4"
	}
	if typ := sniffMimeType(path); typ != "" {
		return typ
	}
	return "application/octet-stream"
}

func sniffMimeType(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	var sample [512]byte
	n, err := file.Read(sample[:])
	if err != nil && n == 0 {
		return ""
	}
	typ := http.DetectContentType(sample[:n])
	if typ == "application/octet-stream" {
		return ""
	}
	return typ
}
