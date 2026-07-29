package media

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var markerTestModTime = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func TestMarkerStoreAddRoundsToWholeSecondsAndKeepsOrder(t *testing.T) {
	dir := t.TempDir()
	store := NewMarkerStore(dir)

	for _, seconds := range []float64{12.4, 3.6, 40} {
		if _, created, err := store.Add("clip.mp4", seconds, "Бодрый Кабачок 7"); err != nil || !created {
			t.Fatalf("expected created marker, got created=%t err=%v", created, err)
		}
	}

	markers, err := store.List("clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 3 {
		t.Fatalf("expected 3 markers, got %#v", markers)
	}
	if markers[0].TimeSeconds != 12 || markers[1].TimeSeconds != 4 || markers[2].TimeSeconds != 40 {
		t.Fatalf("expected whole seconds in insertion order, got %#v", markers)
	}
	if markers[0].Author != "Бодрый Кабачок 7" {
		t.Fatalf("expected saved author, got %#v", markers[0])
	}
	if _, err := os.Stat(filepath.Join(dir, markersDirName, "clip.mp4.jsonl")); err != nil {
		t.Fatalf("expected marker sidecar file, got %v", err)
	}
}

func TestMarkerStoreRefusesRepeatBySameAuthorAtSameSecond(t *testing.T) {
	store := NewMarkerStore(t.TempDir())

	if _, created, err := store.Add("clip.mp4", 8.2, "Тихий Самовар 1"); err != nil || !created {
		t.Fatalf("expected created marker, got created=%t err=%v", created, err)
	}

	marker, created, err := store.Add("clip.mp4", 7.8, "Тихий Самовар 1")
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("expected repeat by the same author to create nothing")
	}
	if marker.TimeSeconds != 8 {
		t.Fatalf("expected existing marker returned, got %#v", marker)
	}

	if _, created, err := store.Add("clip.mp4", 8.0, "Хитрый Вареник 2"); err != nil || !created {
		t.Fatalf("expected another author to mark the same second, got created=%t err=%v", created, err)
	}

	markers, err := store.List("clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 2 {
		t.Fatalf("expected 2 markers, got %#v", markers)
	}
}

func TestMarkerStoreListMissingFileIsEmpty(t *testing.T) {
	store := NewMarkerStore(t.TempDir())

	markers, err := store.List("missing.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 0 {
		t.Fatalf("expected no markers, got %#v", markers)
	}
}

func TestMarkerStoreRejectsInvalidTimes(t *testing.T) {
	store := NewMarkerStore(t.TempDir())

	for _, seconds := range []float64{-1, math.NaN(), math.Inf(1), maxMarkerTimeSecond + 1} {
		if _, _, err := store.Add("clip.mp4", seconds, ""); err == nil {
			t.Fatalf("expected error for marker time %v", seconds)
		}
	}
}

func TestMarkerStoreDefaultsAuthorAndStopsAtLimit(t *testing.T) {
	store := NewMarkerStore(t.TempDir())

	marker, _, err := store.Add("clip.mp4", 0, "   ")
	if err != nil {
		t.Fatal(err)
	}
	if marker.Author != defaultAuthor {
		t.Fatalf("expected default author, got %#v", marker)
	}

	for i := 1; i < maxMarkersPerMedia; i++ {
		if _, created, err := store.Add("clip.mp4", float64(i), ""); err != nil || !created {
			t.Fatalf("expected created marker at %d, got created=%t err=%v", i, created, err)
		}
	}
	if _, _, err := store.Add("clip.mp4", maxMarkersPerMedia+1, ""); !errors.Is(err, ErrMarkerLimitReached) {
		t.Fatalf("expected marker limit error, got %v", err)
	}
}

func TestLibraryAddMarkerUpdatesFeedItemAndRejectsNonVideo(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "clip.mp4", markerTestModTime)
	writeTestFile(t, dir, "photo.png", markerTestModTime)

	library := NewLibrary(dir)
	videoID := EncodeID("clip.mp4")

	if _, created, err := library.AddMarker(videoID, 8.5, "Сонный Пельмень 3"); err != nil || !created {
		t.Fatalf("expected created marker, got created=%t err=%v", created, err)
	}
	if _, created, err := library.AddMarker(videoID, 9.0, "Сонный Пельмень 3"); err != nil || created {
		t.Fatalf("expected repeat to create nothing, got created=%t err=%v", created, err)
	}

	markers, err := library.MarkersForID(videoID)
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 1 || markers[0].TimeSeconds != 9 {
		t.Fatalf("expected one marker at second 9, got %#v", markers)
	}

	item, err := library.ItemForID(videoID)
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Markers) != 1 || item.Markers[0].TimeSeconds != 9 {
		t.Fatalf("expected marker in media item, got %#v", item.Markers)
	}

	if _, _, err := library.AddMarker(EncodeID("photo.png"), 1, ""); err == nil {
		t.Fatal("expected error when marking non-video media")
	}
}

func TestLibraryLoadsMarkersFromDiskForVideoOnly(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "clip.mp4", markerTestModTime)

	if _, _, err := NewMarkerStore(dir).Add("clip.mp4", 4, "Тихий Самовар 1"); err != nil {
		t.Fatal(err)
	}

	item, err := NewLibrary(dir).ItemForID(EncodeID("clip.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Markers) != 1 || item.Markers[0].TimeSeconds != 4 {
		t.Fatalf("expected marker loaded from disk, got %#v", item.Markers)
	}
}
