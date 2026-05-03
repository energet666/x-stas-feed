package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestCommentEndpointsCreateListAndUpdateFeedSummary(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")
	id := media.EncodeID("photo.png")

	handler := New(media.NewLibrary(dir), "", log.New(io.Discard, "", 0)).Handler()
	for _, text := range []string{"first", "second", "third"} {
		req := httptest.NewRequest(http.MethodPost, "/api/media/"+id+"/comments", bytes.NewBufferString(`{"text":"`+text+`","author":"Ламповый Кабачок 42"}`))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()

		handler.ServeHTTP(res, req)

		if res.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d body=%s", res.Code, res.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/"+id+"/comments", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}

	var commentsResponse struct {
		Comments []media.Comment `json:"comments"`
	}
	if err := json.NewDecoder(res.Body).Decode(&commentsResponse); err != nil {
		t.Fatal(err)
	}
	if len(commentsResponse.Comments) != 3 || commentsResponse.Comments[2].Text != "third" {
		t.Fatalf("unexpected comments response: %#v", commentsResponse)
	}
	if commentsResponse.Comments[2].Author != "Ламповый Кабачок 42" {
		t.Fatalf("expected saved comment author, got %#v", commentsResponse.Comments[2])
	}

	req = httptest.NewRequest(http.MethodGet, "/api/feed?limit=1", nil)
	res = httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected feed status 200, got %d", res.Code)
	}

	var page media.Page
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].CommentCount != 3 {
		t.Fatalf("expected feed comment count 3, got %#v", page)
	}
	if len(page.Items[0].Comments) != 2 || page.Items[0].Comments[0].Text != "second" || page.Items[0].Comments[1].Text != "third" {
		t.Fatalf("expected latest two comments in feed summary, got %#v", page.Items[0].Comments)
	}
	if page.Items[0].Comments[1].Author != "Ламповый Кабачок 42" {
		t.Fatalf("expected latest comment author in feed summary, got %#v", page.Items[0].Comments[1])
	}
}

func TestCreateCommentDefaultsAndNormalizesAuthor(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")
	id := media.EncodeID("photo.png")

	handler := New(media.NewLibrary(dir), "", log.New(io.Discard, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/media/"+id+"/comments", bytes.NewBufferString(`{"text":"hello","author":"  Космический\n  Пончик  "}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", res.Code, res.Body.String())
	}

	var comment media.Comment
	if err := json.NewDecoder(res.Body).Decode(&comment); err != nil {
		t.Fatal(err)
	}
	if comment.Author != "Космический Пончик" {
		t.Fatalf("expected normalized author, got %#v", comment)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/media/"+id+"/comments", bytes.NewBufferString(`{"text":"fallback","author":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", res.Code, res.Body.String())
	}
	if err := json.NewDecoder(res.Body).Decode(&comment); err != nil {
		t.Fatal(err)
	}
	if comment.Author != "Guest" {
		t.Fatalf("expected default author, got %#v", comment)
	}
}

func TestCreateCommentRejectsEmptyText(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")

	handler := New(media.NewLibrary(dir), "", log.New(io.Discard, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/media/"+media.EncodeID("photo.png")+"/comments", bytes.NewBufferString(`{"text":"   "}`))
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.Code)
	}
}

func TestCommentEventsStreamsCreatedComments(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")
	id := media.EncodeID("photo.png")

	testServer := httptest.NewServer(New(media.NewLibrary(dir), "", log.New(io.Discard, "", 0)).Handler())
	defer testServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/api/comments/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := testServer.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}
	if contentType := res.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("expected event-stream content type, got %q", contentType)
	}

	lines := make(chan string, 16)
	go func() {
		scanner := bufio.NewScanner(res.Body)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()

	postRes, err := testServer.Client().Post(
		testServer.URL+"/api/media/"+id+"/comments",
		"application/json",
		bytes.NewBufferString(`{"text":"streamed"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = postRes.Body.Close()
	if postRes.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", postRes.StatusCode)
	}

	timeout := time.After(2 * time.Second)
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("comment event stream closed before receiving data")
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var event struct {
				MediaID string        `json:"mediaId"`
				Comment media.Comment `json:"comment"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
				t.Fatal(err)
			}
			if event.MediaID != id || event.Comment.Text != "streamed" {
				t.Fatalf("expected streamed comment event, got %#v", event)
			}
			return
		case <-timeout:
			t.Fatal("timed out waiting for comment event")
		}
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
