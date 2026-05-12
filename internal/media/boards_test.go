package media

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBoardStoreInitCreatesPlaceholderWithBoardCreatedAt(t *testing.T) {
	dir := t.TempDir()
	boardID := "abc123"
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	if err := os.MkdirAll(filepath.Join(dir, boardsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	metaLine, err := json.Marshal(boardMeta{Name: "Sketch", CreatedAt: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, boardsDirName, boardID+".jsonl"), append(metaLine, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dir, boardID+".board"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().UTC().Equal(createdAt) {
		t.Fatalf("expected placeholder mtime to match board creation time, got %s", info.ModTime().UTC())
	}
}
