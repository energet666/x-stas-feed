package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanClassifiesFilesAndSortsByModifiedTime(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

	writeTestFile(t, dir, "old.png", oldTime)
	writeTestFile(t, dir, "new.mp4", newTime)
	writeTestFile(t, dir, "notes.txt", newTime)
	writeTestFile(t, dir, "song.mp3", oldTime)
	writeTestFile(t, dir, ".DS_Store", newTime)

	items, err := NewLibrary(dir).Scan()
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 4 {
		t.Fatalf("expected 4 feed items, got %d", len(items))
	}
	if items[0].Filename != "new.mp4" || items[0].Type != "video" {
		t.Fatalf("expected newest video first, got %#v", items[0])
	}
	if items[0].DisplayName != "new.mp4" {
		t.Fatalf("expected fallback display name, got %#v", items[0])
	}
	if items[1].Filename != "notes.txt" || items[1].Type != "file" {
		t.Fatalf("expected generic file second, got %#v", items[1])
	}
	if items[2].Filename != "old.png" || items[2].Type != "image" {
		t.Fatalf("expected older image second, got %#v", items[1])
	}
	if items[3].Filename != "song.mp3" || items[3].Type != "audio" {
		t.Fatalf("expected mp3 to be classified as audio, got %#v", items[3])
	}
}

func TestScanUsesStoredDisplayName(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "photo.png", modTime)

	library := NewLibrary(dir)
	if err := library.metadata.Set(EncodeID("photo.png"), Metadata{DisplayName: "Летний день 2026.png"}); err != nil {
		t.Fatal(err)
	}

	items, err := library.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].DisplayName != "Летний день 2026.png" {
		t.Fatalf("expected stored display name, got %#v", items)
	}
}

func TestScanUsesStoredLikeCount(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "photo.png", modTime)

	library := NewLibrary(dir)
	if _, err := library.AddLike(EncodeID("photo.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := library.AddLike(EncodeID("photo.png")); err != nil {
		t.Fatal(err)
	}

	items, err := library.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].LikeCount != 2 {
		t.Fatalf("expected stored like count, got %#v", items)
	}
}

func TestScanUsesValidStoredAudioMetadata(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "song.mp3", modTime)
	info, err := os.Stat(filepath.Join(dir, "song.mp3"))
	if err != nil {
		t.Fatal(err)
	}

	library := NewLibrary(dir)
	if err := library.metadata.Set(EncodeID("song.mp3"), Metadata{
		Audio: &AudioMetadata{
			DurationSeconds:       123.456,
			Tags:                  AudioTags{Title: "Cached Title", Artist: "Cached Artist"},
			HasCover:              true,
			CoverFile:             "cached-cover.jpg",
			SourceSize:            info.Size(),
			SourceModTimeUnixNano: info.ModTime().UnixNano(),
		},
	}); err != nil {
		t.Fatal(err)
	}

	items, err := library.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %#v", items)
	}
	if items[0].DurationSeconds != 123.456 {
		t.Fatalf("expected cached duration, got %#v", items[0])
	}
	if items[0].AudioTags == nil || items[0].AudioTags.Title != "Cached Title" || items[0].AudioTags.Artist != "Cached Artist" {
		t.Fatalf("expected cached audio tags, got %#v", items[0].AudioTags)
	}
	if items[0].CoverURL == "" || !items[0].HasCover {
		t.Fatalf("expected cached cover state, got %#v", items[0])
	}
}

func TestScanIgnoresStaleStoredAudioMetadata(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "song.mp3", modTime)

	library := NewLibrary(dir)
	if err := library.metadata.Set(EncodeID("song.mp3"), Metadata{
		Audio: &AudioMetadata{
			DurationSeconds:       123.456,
			Tags:                  AudioTags{Title: "Stale Title"},
			SourceSize:            1,
			SourceModTimeUnixNano: 1,
		},
	}); err != nil {
		t.Fatal(err)
	}

	items, err := library.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %#v", items)
	}
	if items[0].DurationSeconds != 0 || items[0].AudioTags != nil {
		t.Fatalf("expected stale cached audio metadata to be ignored when reprobe fails, got %#v", items[0])
	}
}

func TestScanSortsFilenameTieAscending(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	writeTestFile(t, dir, "b.png", modTime)
	writeTestFile(t, dir, "a.png", modTime)

	items, err := NewLibrary(dir).Scan()
	if err != nil {
		t.Fatal(err)
	}

	if items[0].Filename != "a.png" || items[1].Filename != "b.png" {
		t.Fatalf("expected filename tie-breaker ascending, got %q then %q", items[0].Filename, items[1].Filename)
	}
}

func TestPageUsesCursorAndNextCursor(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "a.png", modTime)
	writeTestFile(t, dir, "b.png", modTime)
	writeTestFile(t, dir, "c.png", modTime)

	page, err := NewLibrary(dir).Page("", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.NextCursor != "2" {
		t.Fatalf("expected first page with next cursor 2, got %#v", page)
	}

	next, err := NewLibrary(dir).Page(page.NextCursor, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Items) != 1 || next.NextCursor != "" {
		t.Fatalf("expected final page with no next cursor, got %#v", next)
	}
}

func TestPageHandlesEmptyAndMissingDirectories(t *testing.T) {
	page, err := NewLibrary(filepath.Join(t.TempDir(), "missing")).Page("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 || page.NextCursor != "" {
		t.Fatalf("expected empty page, got %#v", page)
	}
}

func TestPageDoesNotRescanOutOfBandFilesAfterInitialization(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "initial.png", oldTime)

	library := NewLibrary(dir)
	page, err := library.Page("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Filename != "initial.png" {
		t.Fatalf("expected initial indexed file, got %#v", page)
	}

	writeTestFile(t, dir, "external.png", newTime)

	page, err = library.Page("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Filename != "initial.png" {
		t.Fatalf("expected out-of-band file to stay invisible until restart, got %#v", page)
	}
}

func TestSaveUploadUpdatesRuntimeIndexWithoutRescan(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "initial.png", oldTime)

	library := NewLibrary(dir)
	if _, err := library.Page("", 10); err != nil {
		t.Fatal(err)
	}

	uploaded, err := library.SaveUpload("New Photo.png", strings.NewReader("uploaded image bytes"))
	if err != nil {
		t.Fatal(err)
	}

	page, err := library.Page("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected uploaded item plus initial item, got %#v", page)
	}
	if page.Items[0].ID != uploaded.ID || page.Items[0].DisplayName != "New Photo.png" {
		t.Fatalf("expected uploaded media first with display name, got %#v", page.Items[0])
	}
	if _, _, err := library.PathForID(uploaded.ID); err != nil {
		t.Fatalf("expected uploaded item path lookup to use runtime index: %v", err)
	}
}

func TestSaveUploadAcceptsGenericFiles(t *testing.T) {
	dir := t.TempDir()
	library := NewLibrary(dir)

	uploaded, err := library.SaveUpload("Notes 2026.txt", strings.NewReader("plain text"))
	if err != nil {
		t.Fatal(err)
	}
	if uploaded.Type != "file" || uploaded.MimeType != "text/plain; charset=utf-8" {
		t.Fatalf("expected generic text file upload, got %#v", uploaded)
	}

	page, err := library.Page("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != uploaded.ID {
		t.Fatalf("expected uploaded file in feed, got %#v", page)
	}
}

func TestSaveUploadAcceptsAudioFiles(t *testing.T) {
	dir := t.TempDir()
	library := NewLibrary(dir)

	uploaded, err := library.SaveUpload("Song.mp3", strings.NewReader("not a real mp3 but still an audio feed item"))
	if err != nil {
		t.Fatal(err)
	}
	if uploaded.Type != "audio" {
		t.Fatalf("expected audio upload, got %#v", uploaded)
	}

	page, err := library.Page("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != uploaded.ID || page.Items[0].Type != "audio" {
		t.Fatalf("expected uploaded audio in feed, got %#v", page)
	}
}

func TestSaveUploadSniffsGenericFileMimeType(t *testing.T) {
	dir := t.TempDir()
	library := NewLibrary(dir)

	uploaded, err := library.SaveUpload("README", strings.NewReader("plain text"))
	if err != nil {
		t.Fatal(err)
	}
	if uploaded.Type != "file" || uploaded.MimeType != "text/plain; charset=utf-8" {
		t.Fatalf("expected sniffed text file upload, got %#v", uploaded)
	}
}

func TestSaveUploadPreservesLongDisplayName(t *testing.T) {
	dir := t.TempDir()
	library := NewLibrary(dir)
	longName := strings.Repeat("4d5cda", 35) + ".txt"

	uploaded, err := library.SaveUpload(longName, strings.NewReader("plain text"))
	if err != nil {
		t.Fatal(err)
	}
	if uploaded.DisplayName != longName {
		t.Fatalf("expected full display name to be preserved, got length=%d want length=%d", len([]rune(uploaded.DisplayName)), len([]rune(longName)))
	}

	reloaded := NewLibrary(dir)
	item, err := reloaded.ItemForID(uploaded.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.DisplayName != longName {
		t.Fatalf("expected full stored display name after reload, got length=%d want length=%d", len([]rune(item.DisplayName)), len([]rune(longName)))
	}
}

func TestSaveUploadPreservesDisplayNameWhitespace(t *testing.T) {
	dir := t.TempDir()
	library := NewLibrary(dir)
	originalName := "my   file\tname.txt"

	uploaded, err := library.SaveUpload(originalName, strings.NewReader("plain text"))
	if err != nil {
		t.Fatal(err)
	}
	if uploaded.DisplayName != originalName {
		t.Fatalf("expected exact display name %q, got %q", originalName, uploaded.DisplayName)
	}

	reloaded := NewLibrary(dir)
	item, err := reloaded.ItemForID(uploaded.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.DisplayName != originalName {
		t.Fatalf("expected exact stored display name %q, got %q", originalName, item.DisplayName)
	}
}

func TestSaveUploadWithModifiedAtDisplaysSourceTimeButSortsByServerTime(t *testing.T) {
	dir := t.TempDir()
	library := NewLibrary(dir)
	oldSourceTime := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)

	uploaded, err := library.SaveUploadWithModifiedAt("old-source.txt", strings.NewReader("plain text"), oldSourceTime)
	if err != nil {
		t.Fatal(err)
	}
	if !uploaded.ModifiedAt.Equal(oldSourceTime) {
		t.Fatalf("expected source modified time in uploaded item, got %s", uploaded.ModifiedAt)
	}

	time.Sleep(2 * time.Millisecond)
	writeTestFile(t, dir, "external.png", time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

	reloaded := NewLibrary(dir)
	page, err := reloaded.Page("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected two items, got %#v", page)
	}
	if page.Items[0].ID != uploaded.ID {
		t.Fatalf("expected uploaded item to sort by server file mtime, got %#v", page.Items)
	}
	if !page.Items[0].ModifiedAt.Equal(oldSourceTime) {
		t.Fatalf("expected source modified time to remain visible after reload, got %s", page.Items[0].ModifiedAt)
	}
}

func TestRuntimeIndexUpdatesCommentsActivityAndLikes(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "photo.png", modTime)
	id := EncodeID("photo.png")

	library := NewLibrary(dir)
	if _, err := library.Page("", 10); err != nil {
		t.Fatal(err)
	}

	first, err := library.AddComment(id, "first", "Tester")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	second, err := library.AddComment(id, "second", "Tester")
	if err != nil {
		t.Fatal(err)
	}

	item, err := library.ItemForID(id)
	if err != nil {
		t.Fatal(err)
	}
	if item.CommentCount != 2 || len(item.Comments) != 2 || item.Comments[0].ID != first.ID || item.Comments[1].ID != second.ID {
		t.Fatalf("expected cached comment summary to update, got %#v", item)
	}

	comments, err := library.CommentsForID(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 2 || comments[1].Text != "second" {
		t.Fatalf("expected cached full comments to update, got %#v", comments)
	}

	activity, err := library.Activity(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(activity) != 2 || activity[0].Comment.ID != second.ID || activity[1].Comment.ID != first.ID {
		t.Fatalf("expected cached activity newest first, got %#v", activity)
	}

	likedComment, err := library.AddCommentLike(id, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if likedComment.LikeCount != 1 {
		t.Fatalf("expected liked comment count 1, got %#v", likedComment)
	}

	item, err = library.ItemForID(id)
	if err != nil {
		t.Fatal(err)
	}
	if item.Comments[0].ID != first.ID || item.Comments[0].LikeCount != 1 {
		t.Fatalf("expected comment like in cached summary, got %#v", item.Comments)
	}

	likeCount, err := library.AddLike(id)
	if err != nil {
		t.Fatal(err)
	}
	if likeCount != 1 {
		t.Fatalf("expected media like count 1, got %d", likeCount)
	}
	item, err = library.ItemForID(id)
	if err != nil {
		t.Fatal(err)
	}
	if item.LikeCount != 1 {
		t.Fatalf("expected cached item like count 1, got %#v", item)
	}
}

func TestRuntimeIndexTreatsMissingLongCommentPathAsEmpty(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	nestedDir := filepath.Join(dir, strings.Repeat("deep-", 30), strings.Repeat("nested-", 25))
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, nestedDir, strings.Repeat("media-", 20)+".png", modTime)

	library := NewLibrary(dir)
	page, err := library.Page("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].CommentCount != 0 {
		t.Fatalf("expected long media id to load with empty comments, got %#v", page)
	}
	if page.Items[0].Comments == nil {
		t.Fatal("expected empty comments to be encoded as an empty slice")
	}

	comment, err := library.AddComment(page.Items[0].ID, "works", "Tester")
	if err != nil {
		t.Fatal(err)
	}
	comments, err := library.CommentsForID(page.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || comments[0].ID != comment.ID {
		t.Fatalf("expected comments for long media id to use short storage path, got %#v", comments)
	}
}

func TestPageIgnoresBrokenCommentSummary(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "photo.png", modTime)

	commentsDir := filepath.Join(dir, commentsDirName)
	if err := os.MkdirAll(commentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commentsDir, EncodeID("photo.png")+".jsonl"), []byte("{broken json\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	page, err := NewLibrary(dir).Page("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Filename != "photo.png" {
		t.Fatalf("expected media item despite broken comments, got %#v", page)
	}
	if page.Items[0].CommentCount != 0 || len(page.Items[0].Comments) != 0 {
		t.Fatalf("expected broken summary to be omitted, got %#v", page.Items[0])
	}
}

func TestEmptyCommentFileReturnsEmptyCommentList(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "photo.png", modTime)

	commentsDir := filepath.Join(dir, commentsDirName)
	if err := os.MkdirAll(commentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commentsDir, EncodeID("photo.png")+".jsonl"), []byte("\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	comments, err := NewLibrary(dir).CommentsForID(EncodeID("photo.png"))
	if err != nil {
		t.Fatal(err)
	}
	if comments == nil || len(comments) != 0 {
		t.Fatalf("expected empty comment slice, got %#v", comments)
	}
}

func TestScanIgnoresGeneratedPosterDirectory(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "video.mp4", modTime)
	if err := os.MkdirAll(filepath.Join(dir, posterDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, posterDirName), "poster.jpg", modTime)

	items, err := NewLibrary(dir).Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Filename != "video.mp4" {
		t.Fatalf("expected generated posters to be ignored, got %#v", items)
	}
}

func TestScanIgnoresMetadataDirectory(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "photo.png", modTime)
	if err := os.MkdirAll(filepath.Join(dir, metadataDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, metadataDirName), "metadata.png", modTime)

	items, err := NewLibrary(dir).Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Filename != "photo.png" {
		t.Fatalf("expected metadata files to be ignored, got %#v", items)
	}
}

func TestScanIgnoresGeneratedCoverDirectory(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "song.mp3", modTime)
	if err := os.MkdirAll(filepath.Join(dir, coverDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, coverDirName), "cover.jpg", modTime)

	items, err := NewLibrary(dir).Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Filename != "song.mp3" {
		t.Fatalf("expected generated covers to be ignored, got %#v", items)
	}
}

func TestPathForIDRejectsEscapesAndHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, dir, "ok.png", modTime)
	writeTestFile(t, dir, "notes.txt", modTime)

	path, mimeType, err := NewLibrary(dir).PathForID(EncodeID("ok.png"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "ok.png" || mimeType == "" {
		t.Fatalf("expected png path and mime type, got %q %q", path, mimeType)
	}

	if _, _, err := NewLibrary(dir).PathForID(EncodeID("../secret.png")); err == nil {
		t.Fatal("expected escape id to be rejected")
	}
	path, mimeType, err = NewLibrary(dir).PathForID(EncodeID("notes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "notes.txt" || mimeType == "" {
		t.Fatalf("expected generic file path and mime type, got %q %q", path, mimeType)
	}

	if _, _, err := NewLibrary(dir).PathForID(EncodeID(".comments/notes.txt")); err == nil {
		t.Fatal("expected hidden internal path to be rejected")
	}
}

func writeTestFile(t *testing.T, dir, name string, modTime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}
