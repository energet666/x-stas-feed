package media

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanFiltersAndSortsByModifiedTime(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

	writeTestFile(t, dir, "old.png", oldTime)
	writeTestFile(t, dir, "new.mp4", newTime)
	writeTestFile(t, dir, "notes.txt", newTime)

	items, err := NewLibrary(dir).Scan()
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 supported items, got %d", len(items))
	}
	if items[0].Filename != "new.mp4" || items[0].Type != "video" {
		t.Fatalf("expected newest video first, got %#v", items[0])
	}
	if items[0].DisplayName != "new.mp4" {
		t.Fatalf("expected fallback display name, got %#v", items[0])
	}
	if items[1].Filename != "old.png" || items[1].Type != "image" {
		t.Fatalf("expected older image second, got %#v", items[1])
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

func TestPathForIDRejectsEscapesAndUnsupportedFiles(t *testing.T) {
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
	if _, _, err := NewLibrary(dir).PathForID(EncodeID("notes.txt")); err == nil {
		t.Fatal("expected unsupported file to be rejected")
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
