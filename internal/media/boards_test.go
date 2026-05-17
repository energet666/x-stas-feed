package media

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBoardStoreInitDoesNotCreatePlaceholderFromBoardMetadata(t *testing.T) {
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

	if _, err := os.Stat(filepath.Join(dir, boardID+".board")); !os.IsNotExist(err) {
		t.Fatalf("expected metadata-only board not to create placeholder, err=%v", err)
	}
	if _, err := store.Get(boardID); err != ErrBoardNotFound {
		t.Fatalf("expected metadata-only board not to be loaded, got %v", err)
	}
}

func TestBoardStoreInitLoadsBoardWhenPlaceholderExists(t *testing.T) {
	dir := t.TempDir()
	boardID := "abc123"
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	writeTestFile(t, dir, boardID+".board", createdAt)
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

	info, err := store.Get(boardID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Sketch" || info.MediaID != EncodeID(boardID+".board") {
		t.Fatalf("expected placeholder-backed board to load, got %#v", info)
	}
}

func TestBoardStoreInitCreatesMissingMetadataForPlaceholder(t *testing.T) {
	dir := t.TempDir()
	boardID := "abc123"
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	writeTestFile(t, dir, boardID+".board", createdAt)

	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Get(boardID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != defaultBoardName(boardID) || !info.CreatedAt.Equal(createdAt) {
		t.Fatalf("expected fallback board metadata from placeholder, got %#v", info)
	}
	if _, err := os.Stat(filepath.Join(dir, boardsDirName, boardID+".jsonl")); err != nil {
		t.Fatalf("expected board metadata file to be created: %v", err)
	}
}

func TestBoardStoreAddStrokeNormalizesCoordinates(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Create("Sketch")
	if err != nil {
		t.Fatal(err)
	}

	stroke, err := store.AddStroke(info.ID, "line", [][]float64{
		{-1.234, 40.567},
		{1200.987, 799.949},
	}, "#fff", 4, "Tester")
	if err != nil {
		t.Fatal(err)
	}

	expected := [][]float64{{-1.2, 40.6}, {1201, 799.9}}
	if !samePoints(stroke.Points, expected) {
		t.Fatalf("expected normalized points %#v, got %#v", expected, stroke.Points)
	}

	strokes, err := store.Strokes(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(strokes) != 1 || !samePoints(strokes[0].Points, expected) {
		t.Fatalf("expected stored normalized points %#v, got %#v", expected, strokes)
	}
}

func TestBoardStoreCreateUsesGeneratedDefaultNameForEmptyInput(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Create("")
	if err != nil {
		t.Fatal(err)
	}

	if info.Name == "Board" {
		t.Fatalf("expected generated board name, got %q", info.Name)
	}
	if info.Name != defaultBoardName(info.ID) {
		t.Fatalf("expected name to include board id, got %q for id %q", info.Name, info.ID)
	}
}

func TestBoardStoreCreatePreservesExplicitBoardName(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Create("Board")
	if err != nil {
		t.Fatal(err)
	}

	if info.Name != "Board" {
		t.Fatalf("expected explicit board name to be preserved, got %q", info.Name)
	}
}

func TestBoardStoreAddStrokeAllowsSingleFreeformPoint(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Create("Sketch")
	if err != nil {
		t.Fatal(err)
	}

	stroke, err := store.AddStroke(info.ID, "freeform", [][]float64{{10.04, 20.06}}, "#fff", 4, "Tester")
	if err != nil {
		t.Fatal(err)
	}

	expected := [][]float64{{10, 20.1}}
	if !samePoints(stroke.Points, expected) {
		t.Fatalf("expected normalized point %#v, got %#v", expected, stroke.Points)
	}
}

func TestBoardStoreAddStrokeRejectsSingleLinePoint(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Create("Sketch")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.AddStroke(info.ID, "line", [][]float64{{10, 20}}, "#fff", 4, "Tester"); err == nil {
		t.Fatal("expected single-point line stroke to be rejected")
	}
}

func TestBoardStoreAddStrokeRejectsInvalidPoints(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Create("Sketch")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.AddStroke(info.ID, "line", [][]float64{{1, 2, 3}, {4, 5}}, "#fff", 4, "Tester"); err == nil {
		t.Fatal("expected invalid coordinate pair to be rejected")
	}
}

func samePoints(a, b [][]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}
