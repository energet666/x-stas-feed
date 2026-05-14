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
