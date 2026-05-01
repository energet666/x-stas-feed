package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"feed-ai/internal/media"
)

func TestFeedEndpointReturnsPage(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")

	handler := New(media.NewLibrary(dir), "", log.New(io.Discard, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/feed?limit=1", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}

	var page media.Page
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Filename != "photo.png" {
		t.Fatalf("unexpected page: %#v", page)
	}
}

func TestMediaEndpointServesKnownIDAndRejectsEscape(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")

	handler := New(media.NewLibrary(dir), "", log.New(io.Discard, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodGet, "/media/"+media.EncodeID("photo.png"), nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/media/"+media.EncodeID("../secret.png"), nil)
	res = httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for path escape, got %d", res.Code)
	}
}

func writeServerTestFile(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	modTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}
