package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
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
	if cacheControl := res.Header().Get("Cache-Control"); cacheControl != mediaCacheControl {
		t.Fatalf("expected media Cache-Control %q, got %q", mediaCacheControl, cacheControl)
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

func TestLikeEndpointIncrementsMetadataAndFeedSummary(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")
	id := media.EncodeID("photo.png")

	handler := New(media.NewLibrary(dir), "", log.New(io.Discard, "", 0)).Handler()
	for expected := 1; expected <= 2; expected++ {
		req := httptest.NewRequest(http.MethodPost, "/api/media/"+id+"/likes", nil)
		res := httptest.NewRecorder()

		handler.ServeHTTP(res, req)

		if res.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d body=%s", res.Code, res.Body.String())
		}

		var response struct {
			LikeCount int `json:"likeCount"`
		}
		if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
			t.Fatal(err)
		}
		if response.LikeCount != expected {
			t.Fatalf("expected like count %d, got %#v", expected, response)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/feed?limit=1", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected feed status 200, got %d", res.Code)
	}

	var page media.Page
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].LikeCount != 2 {
		t.Fatalf("expected feed like count 2, got %#v", page)
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

func TestCommentEventsStreamsCreatedLikes(t *testing.T) {
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

	lines := make(chan string, 16)
	go func() {
		scanner := bufio.NewScanner(res.Body)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()

	postRes, err := testServer.Client().Post(
		testServer.URL+"/api/media/"+id+"/likes",
		"application/json",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = postRes.Body.Close()
	if postRes.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", postRes.StatusCode)
	}

	timeout := time.After(2 * time.Second)
	seenLikeEvent := false
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("event stream closed before receiving like data")
			}
			if line == "event: like" {
				seenLikeEvent = true
				continue
			}
			if !seenLikeEvent || !strings.HasPrefix(line, "data: ") {
				continue
			}
			var event struct {
				MediaID   string `json:"mediaId"`
				LikeCount int    `json:"likeCount"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
				t.Fatal(err)
			}
			if event.MediaID != id || event.LikeCount != 1 {
				t.Fatalf("expected streamed like event, got %#v", event)
			}
			return
		case <-timeout:
			t.Fatal("timed out waiting for like event")
		}
	}
}

func TestUploadEndpointSavesMediaAndRefreshesFeed(t *testing.T) {
	dir := t.TempDir()
	handler := New(media.NewLibrary(dir), "", log.New(io.Discard, "", 0)).Handler()

	req := newUploadRequest(t, "Мой летний день.png", []byte("png-content"))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", res.Code, res.Body.String())
	}

	var upload uploadResponse
	if err := json.NewDecoder(res.Body).Decode(&upload); err != nil {
		t.Fatal(err)
	}
	if len(upload.Items) != 1 {
		t.Fatalf("expected one uploaded item, got %#v", upload)
	}
	item := upload.Items[0]
	if item.Type != "image" || item.Size != int64(len("png-content")) {
		t.Fatalf("unexpected uploaded item: %#v", item)
	}
	if item.DisplayName != "Мой летний день.png" {
		t.Fatalf("expected original display name, got %#v", item)
	}
	if strings.Contains(item.Filename, "Мой") || strings.Contains(item.Filename, " ") {
		t.Fatalf("expected safe technical filename, got %#v", item.Filename)
	}
	if _, err := os.Stat(filepath.Join(dir, item.Filename)); err != nil {
		t.Fatal(err)
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
	if len(page.Items) != 1 || page.Items[0].ID != item.ID {
		t.Fatalf("expected uploaded item in feed, got %#v", page)
	}

	req = httptest.NewRequest(http.MethodGet, "/media/"+item.ID, nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK || res.Body.String() != "png-content" {
		t.Fatalf("expected uploaded media response, got status=%d body=%q", res.Code, res.Body.String())
	}
}

func TestUploadEndpointSavesMultipleFilesWithUniqueNames(t *testing.T) {
	dir := t.TempDir()
	handler := New(media.NewLibrary(dir), "", log.New(io.Discard, "", 0)).Handler()

	req := newUploadRequest(t, "clip.mp4", []byte("video"), "clip.mp4", []byte("video-two"))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", res.Code, res.Body.String())
	}

	var upload uploadResponse
	if err := json.NewDecoder(res.Body).Decode(&upload); err != nil {
		t.Fatal(err)
	}
	if len(upload.Items) != 2 {
		t.Fatalf("expected two uploaded items, got %#v", upload)
	}
	if upload.Items[0].Filename == upload.Items[1].Filename {
		t.Fatalf("expected unique generated filenames, got %#v", upload.Items)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/feed?limit=10", nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	var page media.Page
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected both uploaded files in feed, got %#v", page)
	}
}

func TestUploadEndpointRejectsInvalidUploads(t *testing.T) {
	tests := []struct {
		name string
		req  func(t *testing.T) *http.Request
	}{
		{
			name: "unsupported extension",
			req:  func(t *testing.T) *http.Request { return newUploadRequest(t, "notes.txt", []byte("text")) },
		},
		{
			name: "empty file",
			req:  func(t *testing.T) *http.Request { return newUploadRequest(t, "empty.png", []byte{}) },
		},
		{
			name: "path filename",
			req:  func(t *testing.T) *http.Request { return newUploadRequest(t, "../photo.png", []byte("png")) },
		},
		{
			name: "no files",
			req:  newEmptyUploadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := New(media.NewLibrary(t.TempDir()), "", log.New(io.Discard, "", 0)).Handler()
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, tt.req(t))

			if res.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d body=%s", res.Code, res.Body.String())
			}
		})
	}
}

func TestUploadEndpointRejectsKnownOversizedRequest(t *testing.T) {
	handler := New(media.NewLibrary(t.TempDir()), "", log.New(io.Discard, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/uploads", strings.NewReader(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	req.ContentLength = uploadMaxBytes + 1
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d body=%s", res.Code, res.Body.String())
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

func newUploadRequest(t *testing.T, files ...any) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for i := 0; i < len(files); i += 2 {
		filename := files[i].(string)
		content := files[i+1].([]byte)
		part, err := writer.CreateFormFile("files", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func newEmptyUploadRequest(t *testing.T) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}
