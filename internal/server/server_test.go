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
	"strconv"
	"strings"
	"testing"
	"time"

	"feed-ai/internal/media"
)

func TestFeedEndpointReturnsPage(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
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

func TestFavoriteFeedEndpointReturnsIDsInRequestedOrder(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "a.png")
	writeServerTestFile(t, dir, "b.png")
	writeServerTestFile(t, dir, "c.png")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
	body := bytes.NewBufferString(`{"ids":["` + media.EncodeID("c.png") + `","` + media.EncodeID("a.png") + `","` + media.EncodeID("b.png") + `"],"limit":10}`)
	req := httptest.NewRequest(http.MethodPost, "/api/feed/favorites", body)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}

	var page media.Page
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 3 {
		t.Fatalf("expected three favorite items, got %#v", page)
	}
	if page.Items[0].Filename != "c.png" || page.Items[1].Filename != "a.png" || page.Items[2].Filename != "b.png" {
		t.Fatalf("expected requested favorite order, got %#v", page.Items)
	}
}

func TestFavoriteFeedEndpointCursorLimitAndStaleIDs(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "a.png")
	writeServerTestFile(t, dir, "b.png")
	writeServerTestFile(t, dir, "c.png")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
	missingID := media.EncodeID("missing.png")
	body := bytes.NewBufferString(`{"ids":["` + missingID + `","` + media.EncodeID("c.png") + `","` + media.EncodeID("a.png") + `","` + media.EncodeID("b.png") + `"],"limit":2}`)
	req := httptest.NewRequest(http.MethodPost, "/api/feed/favorites", body)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}

	var page media.Page
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].Filename != "c.png" || page.Items[1].Filename != "a.png" {
		t.Fatalf("expected first page to skip stale id and preserve order, got %#v", page)
	}
	if page.NextCursor != "3" {
		t.Fatalf("expected next cursor 3, got %q", page.NextCursor)
	}

	body = bytes.NewBufferString(`{"ids":["` + missingID + `","` + media.EncodeID("c.png") + `","` + media.EncodeID("a.png") + `","` + media.EncodeID("b.png") + `"],"cursor":"` + page.NextCursor + `","limit":2}`)
	req = httptest.NewRequest(http.MethodPost, "/api/feed/favorites", body)
	req.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	page = media.Page{}
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Filename != "b.png" || page.NextCursor != "" {
		t.Fatalf("expected final favorite page, got %#v", page)
	}
}

func TestActivityEndpointReturnsLatestCommentsAcrossMedia(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "a.png")
	writeServerTestFile(t, dir, "b.png")
	aID := media.EncodeID("a.png")
	bID := media.EncodeID("b.png")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
	for _, request := range []struct {
		id   string
		text string
	}{
		{id: aID, text: "first"},
		{id: bID, text: "second"},
		{id: aID, text: "third"},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/media/"+request.id+"/comments", bytes.NewBufferString(`{"text":"`+request.text+`","author":"Tester"}`))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d body=%s", res.Code, res.Body.String())
		}
		time.Sleep(time.Millisecond)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/activity?limit=2", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Items []media.ActivityItem `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 2 {
		t.Fatalf("expected two activity items, got %#v", response)
	}
	if response.Items[0].Comment.Text != "third" || response.Items[1].Comment.Text != "second" {
		t.Fatalf("expected newest comments first, got %#v", response.Items)
	}
	if response.Items[0].MediaID != aID || response.Items[0].MediaDisplayName != "a.png" || response.Items[0].MediaType != "image" {
		t.Fatalf("expected media metadata on activity item, got %#v", response.Items[0])
	}
}

func TestActivityEndpointIgnoresStaleCommentFilesAndCapsLimit(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")
	id := media.EncodeID("photo.png")
	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()

	for i := 0; i < 105; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/media/"+id+"/comments", bytes.NewBufferString(`{"text":"comment"}`))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d body=%s", res.Code, res.Body.String())
		}
	}

	staleID := media.EncodeID("missing.png")
	commentsDir := filepath.Join(dir, ".comments")
	if err := os.MkdirAll(commentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commentsDir, staleID+".jsonl"), []byte(`{"id":"stale","author":"Ghost","text":"stale","createdAt":"2099-01-01T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/activity?limit=1000", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Items []media.ActivityItem `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 100 {
		t.Fatalf("expected activity limit cap at 100, got %d", len(response.Items))
	}
	for _, item := range response.Items {
		if item.MediaID == staleID || item.Comment.ID == "stale" {
			t.Fatalf("expected stale comment file to be ignored, got %#v", item)
		}
	}
}

func TestMediaItemEndpointReturnsItemWithCommentSummary(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")
	id := media.EncodeID("photo.png")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
	for _, text := range []string{"first", "second", "third"} {
		req := httptest.NewRequest(http.MethodPost, "/api/media/"+id+"/comments", bytes.NewBufferString(`{"text":"`+text+`"}`))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d body=%s", res.Code, res.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/"+id, nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}

	var item media.Item
	if err := json.NewDecoder(res.Body).Decode(&item); err != nil {
		t.Fatal(err)
	}
	if item.ID != id || item.Filename != "photo.png" || item.CommentCount != 3 {
		t.Fatalf("unexpected media item response: %#v", item)
	}
	if len(item.Comments) != 2 || item.Comments[0].Text != "second" || item.Comments[1].Text != "third" {
		t.Fatalf("expected latest two comment summary, got %#v", item.Comments)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/media/"+media.EncodeID("../secret.png"), nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for invalid id, got %d", res.Code)
	}
}

func TestMediaCoverEndpointRejectsInvalidAndNonAudioIDs(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")
	writeServerTestFile(t, dir, "song.mp3")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()

	for _, path := range []string{
		"/api/media/" + media.EncodeID("../secret.mp3") + "/cover",
		"/api/media/" + media.EncodeID("photo.png") + "/cover",
		"/api/media/" + media.EncodeID("song.mp3") + "/cover",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusNotFound {
			t.Fatalf("expected cover request %s to return 404, got %d body=%s", path, res.Code, res.Body.String())
		}
	}
}

func TestMediaEndpointServesKnownIDAndRejectsEscape(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
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

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
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

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
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

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
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

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
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

func TestCommentLikeEndpointIncrementsCommentAndFeedSummary(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")
	id := media.EncodeID("photo.png")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/media/"+id+"/comments", bytes.NewBufferString(`{"text":"liked comment"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected comment status 201, got %d body=%s", res.Code, res.Body.String())
	}

	var comment media.Comment
	if err := json.NewDecoder(res.Body).Decode(&comment); err != nil {
		t.Fatal(err)
	}

	for expected := 1; expected <= 2; expected++ {
		req = httptest.NewRequest(http.MethodPost, "/api/media/"+id+"/comments/"+comment.ID+"/likes", nil)
		res = httptest.NewRecorder()

		handler.ServeHTTP(res, req)

		if res.Code != http.StatusCreated {
			t.Fatalf("expected comment like status 201, got %d body=%s", res.Code, res.Body.String())
		}

		var response struct {
			LikeCount int `json:"likeCount"`
		}
		if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
			t.Fatal(err)
		}
		if response.LikeCount != expected {
			t.Fatalf("expected comment like count %d, got %#v", expected, response)
		}
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
	if len(page.Items) != 1 || len(page.Items[0].Comments) != 1 || page.Items[0].Comments[0].LikeCount != 2 {
		t.Fatalf("expected feed comment like count 2, got %#v", page)
	}
}

func TestCommentEventsStreamsCreatedComments(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")
	id := media.EncodeID("photo.png")

	testServer := httptest.NewServer(New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler())
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

	testServer := httptest.NewServer(New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler())
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

func TestCommentEventsStreamsCreatedCommentLikes(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")
	id := media.EncodeID("photo.png")

	testServer := httptest.NewServer(New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler())
	defer testServer.Close()

	postCommentRes, err := testServer.Client().Post(
		testServer.URL+"/api/media/"+id+"/comments",
		"application/json",
		bytes.NewBufferString(`{"text":"streamed like"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	var comment media.Comment
	if err := json.NewDecoder(postCommentRes.Body).Decode(&comment); err != nil {
		t.Fatal(err)
	}
	_ = postCommentRes.Body.Close()
	if postCommentRes.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", postCommentRes.StatusCode)
	}

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

	lines := make(chan string, 16)
	go func() {
		scanner := bufio.NewScanner(res.Body)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()

	postLikeRes, err := testServer.Client().Post(
		testServer.URL+"/api/media/"+id+"/comments/"+comment.ID+"/likes",
		"application/json",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = postLikeRes.Body.Close()
	if postLikeRes.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", postLikeRes.StatusCode)
	}

	timeout := time.After(2 * time.Second)
	seenCommentLikeEvent := false
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("event stream closed before receiving comment like data")
			}
			if line == "event: comment-like" {
				seenCommentLikeEvent = true
				continue
			}
			if !seenCommentLikeEvent || !strings.HasPrefix(line, "data: ") {
				continue
			}
			var event struct {
				MediaID   string `json:"mediaId"`
				CommentID string `json:"commentId"`
				LikeCount int    `json:"likeCount"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
				t.Fatal(err)
			}
			if event.MediaID != id || event.CommentID != comment.ID || event.LikeCount != 1 {
				t.Fatalf("expected streamed comment like event, got %#v", event)
			}
			return
		case <-timeout:
			t.Fatal("timed out waiting for comment like event")
		}
	}
}

func TestUploadEndpointSavesMediaAndRefreshesFeed(t *testing.T) {
	dir := t.TempDir()
	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()

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
	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()

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

func TestUploadEndpointSavesGenericFile(t *testing.T) {
	dir := t.TempDir()
	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()

	req := newUploadRequest(t, "notes.txt", []byte("text"))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", res.Code, res.Body.String())
	}

	var upload uploadResponse
	if err := json.NewDecoder(res.Body).Decode(&upload); err != nil {
		t.Fatal(err)
	}
	if len(upload.Items) != 1 || upload.Items[0].Type != "file" {
		t.Fatalf("expected uploaded generic file item, got %#v", upload)
	}
}

func TestUploadEndpointStoresClientModifiedAt(t *testing.T) {
	dir := t.TempDir()
	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
	sourceModifiedAt := time.Date(2022, 3, 4, 5, 6, 7, 0, time.UTC)

	req := newUploadRequestWithModifiedAt(t, "notes.txt", []byte("text"), sourceModifiedAt)
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
		t.Fatalf("expected uploaded item, got %#v", upload)
	}
	if !upload.Items[0].ModifiedAt.Equal(sourceModifiedAt) {
		t.Fatalf("expected client modified time %s, got %s", sourceModifiedAt, upload.Items[0].ModifiedAt)
	}
}

func TestUploadEndpointRejectsInvalidUploads(t *testing.T) {
	tests := []struct {
		name string
		req  func(t *testing.T) *http.Request
	}{
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
			tmpDir := t.TempDir()
			handler := New(media.NewLibrary(tmpDir), tmpDir, "", log.New(io.Discard, "", 0)).Handler()
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, tt.req(t))

			if res.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d body=%s", res.Code, res.Body.String())
			}
		})
	}
}

func TestUploadEndpointRejectsKnownOversizedRequest(t *testing.T) {
	tmpDir := t.TempDir()
	handler := New(media.NewLibrary(tmpDir), tmpDir, "", log.New(io.Discard, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/uploads", strings.NewReader(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	req.ContentLength = uploadMaxBytes + 1
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestCreateBoardIndexesOpaqueMediaItemForComments(t *testing.T) {
	dir := t.TempDir()
	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/boards", bytes.NewBufferString(`{"name":"Sketch"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", res.Code, res.Body.String())
	}

	var board media.BoardInfo
	if err := json.NewDecoder(res.Body).Decode(&board); err != nil {
		t.Fatal(err)
	}
	if board.ID == "" || board.MediaID == "" || board.MediaID == board.ID {
		t.Fatalf("expected distinct board and media ids, got %#v", board)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/media/"+board.MediaID+"/comments", bytes.NewBufferString(`{"text":"works","author":"Tester"}`))
	req.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected board comment to be accepted, got %d body=%s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/feed?limit=10", nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	var page media.Page
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != board.MediaID || page.Items[0].BoardID != board.ID || page.Items[0].CommentCount != 1 {
		t.Fatalf("expected indexed board media item with comment summary, got %#v", page)
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

func newUploadRequestWithModifiedAt(t *testing.T, filename string, content []byte, modifiedAt time.Time) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("modifiedAt", strconv.FormatInt(modifiedAt.UnixMilli(), 10)); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("files", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
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
