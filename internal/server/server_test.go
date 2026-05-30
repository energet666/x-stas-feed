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

func TestFeedEndpointReturnsIndexedItem(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.txt")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/feed?index=-1", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}

	var response media.IndexedItem
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Index != 0 || response.Item.Filename != "photo.txt" {
		t.Fatalf("unexpected feed item: %#v", response)
	}
}

func TestFeedScanKeepsImagesInMediaRootWithBoardHistory(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.png")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/feed?index=-1", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response media.IndexedItem
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Item.Type != "image" || response.Item.DisplayName != "photo.png" {
		t.Fatalf("expected scanned image media item, got %#v", response.Item)
	}
	if _, err := os.Stat(filepath.Join(dir, "photo.png")); err != nil {
		t.Fatalf("expected original image to remain in media root: %v", err)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, "*.board")); err != nil || len(matches) != 0 {
		t.Fatalf("expected no board placeholder for image, matches=%v err=%v", matches, err)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".boards", "*_bgimg.png")); err != nil || len(matches) != 0 {
		t.Fatalf("expected no copied image background, matches=%v err=%v", matches, err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/boards/"+response.Item.ID, nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected image board response, got status=%d body=%q", res.Code, res.Body.String())
	}
	var boardData struct {
		Board media.BoardInfo `json:"board"`
	}
	if err := json.NewDecoder(res.Body).Decode(&boardData); err != nil {
		t.Fatal(err)
	}
	if boardData.Board.ID != response.Item.ID || boardData.Board.MediaID != response.Item.ID {
		t.Fatalf("expected image media id to own board history, got %#v", boardData.Board)
	}
	if boardData.Board.Background == nil || boardData.Board.Background.URL != response.Item.URL {
		t.Fatalf("expected root media URL as board background, got %#v", boardData.Board)
	}
	if _, err := os.Stat(filepath.Join(dir, ".boards", "photo.png.jsonl")); err != nil {
		t.Fatalf("expected board history file for image media: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/boards/"+response.Item.ID+"/strokes", bytes.NewBufferString(`{"tool":"freeform","points":[[1,2]],"color":"#fff","size":4,"author":"Tester"}`))
	req.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected image board stroke status 204, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestFeedScanKeepsGIFAsImageMedia(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "animated.gif")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/feed?index=-1", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response media.IndexedItem
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Item.Type != "image" || response.Item.Filename != "animated.gif" {
		t.Fatalf("expected scanned gif to remain image media, got %#v", response.Item)
	}
	if _, err := os.Stat(filepath.Join(dir, "animated.gif")); err != nil {
		t.Fatalf("expected original gif to remain in media root: %v", err)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, "*.board")); err != nil || len(matches) != 0 {
		t.Fatalf("expected no board placeholder for gif, matches=%v err=%v", matches, err)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".boards", "*_bgimg.gif")); err != nil || len(matches) != 0 {
		t.Fatalf("expected no board background for gif, matches=%v err=%v", matches, err)
	}
}

func TestRequestLogIncludesQueryStatusBytesAndDuration(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.txt")

	var logs bytes.Buffer
	handler := New(media.NewLibrary(dir), dir, "", log.New(&logs, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/feed?index=-1&limit=2", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	output := logs.String()
	if !strings.Contains(output, `path="/api/feed" query="index=-1&limit=2"`) {
		t.Fatalf("expected request log to include query string, got %q", output)
	}
	if !strings.Contains(output, "status=200") || !strings.Contains(output, "bytes=") || !strings.Contains(output, "duration=") {
		t.Fatalf("expected request log to include status, bytes, and duration, got %q", output)
	}
}

func TestMediaRequestLogIncludesFilename(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.txt")
	id := media.EncodeID("photo.txt")

	var logs bytes.Buffer
	handler := New(media.NewLibrary(dir), dir, "", log.New(&logs, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodGet, "/media/"+id, nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	output := logs.String()
	if !strings.Contains(output, "mediaID="+id) || !strings.Contains(output, `filename="photo.txt"`) {
		t.Fatalf("expected media request log to include media id and filename, got %q", output)
	}
}

func TestFavoriteFeedEndpointReturnsIDsInRequestedOrder(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "a.txt")
	writeServerTestFile(t, dir, "b.txt")
	writeServerTestFile(t, dir, "c.txt")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
	body := bytes.NewBufferString(`{"ids":["` + media.EncodeID("c.txt") + `","` + media.EncodeID("a.txt") + `","` + media.EncodeID("b.txt") + `"],"limit":10}`)
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
	if page.Items[0].Filename != "c.txt" || page.Items[1].Filename != "a.txt" || page.Items[2].Filename != "b.txt" {
		t.Fatalf("expected requested favorite order, got %#v", page.Items)
	}
}

func TestFavoriteFeedEndpointCursorLimitAndStaleIDs(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "a.txt")
	writeServerTestFile(t, dir, "b.txt")
	writeServerTestFile(t, dir, "c.txt")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
	missingID := media.EncodeID("missing.png")
	body := bytes.NewBufferString(`{"ids":["` + missingID + `","` + media.EncodeID("c.txt") + `","` + media.EncodeID("a.txt") + `","` + media.EncodeID("b.txt") + `"],"limit":2}`)
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
	if len(page.Items) != 2 || page.Items[0].Filename != "c.txt" || page.Items[1].Filename != "a.txt" {
		t.Fatalf("expected first page to skip stale id and preserve order, got %#v", page)
	}
	if page.NextCursor != "3" {
		t.Fatalf("expected next cursor 3, got %q", page.NextCursor)
	}

	body = bytes.NewBufferString(`{"ids":["` + missingID + `","` + media.EncodeID("c.txt") + `","` + media.EncodeID("a.txt") + `","` + media.EncodeID("b.txt") + `"],"cursor":"` + page.NextCursor + `","limit":2}`)
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
	if len(page.Items) != 1 || page.Items[0].Filename != "b.txt" || page.NextCursor != "" {
		t.Fatalf("expected final favorite page, got %#v", page)
	}
}

func TestShipScoresEndpointKeepsTopFive(t *testing.T) {
	dir := t.TempDir()
	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()

	for _, score := range []int{100, 600, -200, 400, 800, 300} {
		req := httptest.NewRequest(http.MethodPost, "/api/ships/scores", bytes.NewBufferString(`{"name":"Pilot","score":`+strconv.Itoa(score)+`}`))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d body=%s", res.Code, res.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ships/scores", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Scores []media.GameScore `json:"scores"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	want := []int{800, 600, 400, 300, 100}
	if len(response.Scores) != len(want) {
		t.Fatalf("expected scores %#v, got %#v", want, response.Scores)
	}
	for i, score := range response.Scores {
		if score.Score != want[i] {
			t.Fatalf("score %d: expected %d, got %#v", i, want[i], response.Scores)
		}
	}
}

func TestActivityEndpointReturnsLatestCommentsAcrossMedia(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "a.txt")
	writeServerTestFile(t, dir, "b.txt")
	aID := media.EncodeID("a.txt")
	bID := media.EncodeID("b.txt")

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
	if response.Items[0].MediaID != aID || response.Items[0].MediaDisplayName != "a.txt" || response.Items[0].MediaType != "file" {
		t.Fatalf("expected media metadata on activity item, got %#v", response.Items[0])
	}
}

func TestActivityEndpointIgnoresStaleCommentFilesAndCapsLimit(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.txt")
	id := media.EncodeID("photo.txt")
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
	writeServerTestFile(t, dir, "photo.txt")
	id := media.EncodeID("photo.txt")

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
	if item.ID != id || item.Filename != "photo.txt" || item.CommentCount != 3 {
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
	writeServerTestFile(t, dir, "photo.txt")
	writeServerTestFile(t, dir, "song.mp3")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()

	for _, path := range []string{
		"/api/media/" + media.EncodeID("../secret.mp3") + "/cover",
		"/api/media/" + media.EncodeID("photo.txt") + "/cover",
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
	writeServerTestFile(t, dir, "photo.txt")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodGet, "/media/"+media.EncodeID("photo.txt"), nil)
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
	writeServerTestFile(t, dir, "photo.txt")
	id := media.EncodeID("photo.txt")

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

	req = httptest.NewRequest(http.MethodGet, "/api/feed?index=-1", nil)
	res = httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected feed status 200, got %d", res.Code)
	}

	var feedItem media.IndexedItem
	if err := json.NewDecoder(res.Body).Decode(&feedItem); err != nil {
		t.Fatal(err)
	}
	if feedItem.Item.CommentCount != 3 {
		t.Fatalf("expected feed comment count 3, got %#v", feedItem)
	}
	if len(feedItem.Item.Comments) != 2 || feedItem.Item.Comments[0].Text != "second" || feedItem.Item.Comments[1].Text != "third" {
		t.Fatalf("expected latest two comments in feed summary, got %#v", feedItem.Item.Comments)
	}
	if feedItem.Item.Comments[1].Author != "Ламповый Кабачок 42" {
		t.Fatalf("expected latest comment author in feed summary, got %#v", feedItem.Item.Comments[1])
	}
}

func TestCreateCommentDefaultsAndNormalizesAuthor(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.txt")
	id := media.EncodeID("photo.txt")

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
	writeServerTestFile(t, dir, "photo.txt")

	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/media/"+media.EncodeID("photo.txt")+"/comments", bytes.NewBufferString(`{"text":"   "}`))
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.Code)
	}
}

func TestLikeEndpointIncrementsMetadataAndFeedSummary(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.txt")
	id := media.EncodeID("photo.txt")

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

	req := httptest.NewRequest(http.MethodGet, "/api/feed?index=-1", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected feed status 200, got %d", res.Code)
	}

	var feedItem media.IndexedItem
	if err := json.NewDecoder(res.Body).Decode(&feedItem); err != nil {
		t.Fatal(err)
	}
	if feedItem.Item.LikeCount != 2 {
		t.Fatalf("expected feed like count 2, got %#v", feedItem)
	}
}

func TestCommentLikeEndpointIncrementsCommentAndFeedSummary(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.txt")
	id := media.EncodeID("photo.txt")

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

	req = httptest.NewRequest(http.MethodGet, "/api/feed?index=-1", nil)
	res = httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected feed status 200, got %d", res.Code)
	}

	var feedItem media.IndexedItem
	if err := json.NewDecoder(res.Body).Decode(&feedItem); err != nil {
		t.Fatal(err)
	}
	if len(feedItem.Item.Comments) != 1 || feedItem.Item.Comments[0].LikeCount != 2 {
		t.Fatalf("expected feed comment like count 2, got %#v", feedItem)
	}
}

func TestCommentEventsStreamsCreatedComments(t *testing.T) {
	dir := t.TempDir()
	writeServerTestFile(t, dir, "photo.txt")
	id := media.EncodeID("photo.txt")

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
	writeServerTestFile(t, dir, "photo.txt")
	id := media.EncodeID("photo.txt")

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
	writeServerTestFile(t, dir, "photo.txt")
	id := media.EncodeID("photo.txt")

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

func TestCommentEventsStreamsUploadedFeedItems(t *testing.T) {
	dir := t.TempDir()

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

	uploadReq := newUploadRequest(t, "new.png", []byte("png-content"))
	uploadReq.URL.Scheme = "http"
	uploadReq.URL.Host = testServer.Listener.Addr().String()
	uploadReq.RequestURI = ""
	uploadRes, err := testServer.Client().Do(uploadReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = uploadRes.Body.Close()
	if uploadRes.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", uploadRes.StatusCode)
	}

	timeout := time.After(2 * time.Second)
	seenFeedItemEvent := false
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("feed event stream closed before receiving data")
			}
			if line == "event: feed-item-created" {
				seenFeedItemEvent = true
				continue
			}
			if !seenFeedItemEvent || !strings.HasPrefix(line, "data: ") {
				continue
			}
			var event struct {
				Index     int        `json:"index"`
				LastIndex int        `json:"lastIndex"`
				Item      media.Item `json:"item"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
				t.Fatal(err)
			}
			if event.Index != 0 || event.LastIndex != 0 || event.Item.DisplayName != "new.png" {
				t.Fatalf("expected uploaded media event, got %#v", event)
			}
			return
		case <-timeout:
			t.Fatal("timed out waiting for feed item event")
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
	if item.Type != "image" {
		t.Fatalf("unexpected uploaded item: %#v", item)
	}
	if item.DisplayName != "Мой летний день.png" {
		t.Fatalf("expected original display name, got %#v", item)
	}
	if _, err := os.Stat(filepath.Join(dir, item.Filename)); err != nil {
		t.Fatal(err)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, "*.board")); err != nil || len(matches) != 0 {
		t.Fatalf("expected no board placeholder for uploaded image, matches=%v err=%v", matches, err)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".boards", "*_bgimg.png")); err != nil || len(matches) != 0 {
		t.Fatalf("expected no copied board background for uploaded image, matches=%v err=%v", matches, err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/feed?index=-1", nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected feed status 200, got %d", res.Code)
	}

	var feedItem media.IndexedItem
	if err := json.NewDecoder(res.Body).Decode(&feedItem); err != nil {
		t.Fatal(err)
	}
	if feedItem.Item.ID != item.ID {
		t.Fatalf("expected uploaded item in feed, got %#v", feedItem)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/boards/"+item.ID, nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected uploaded board response, got status=%d body=%q", res.Code, res.Body.String())
	}
	var boardData struct {
		Board media.BoardInfo `json:"board"`
	}
	if err := json.NewDecoder(res.Body).Decode(&boardData); err != nil {
		t.Fatal(err)
	}
	if boardData.Board.Background == nil || boardData.Board.Background.Type != "image" || boardData.Board.Background.URL == "" {
		t.Fatalf("expected board image background, got %#v", boardData.Board)
	}
	if boardData.Board.ID != item.ID || boardData.Board.MediaID != item.ID || boardData.Board.Background.URL != item.URL {
		t.Fatalf("expected uploaded image media to own board history, got %#v", boardData.Board)
	}
	if _, err := os.Stat(filepath.Join(dir, ".boards", item.Filename+".jsonl")); err != nil {
		t.Fatalf("expected uploaded image board history file: %v", err)
	}
}

func TestUploadEndpointKeepsGIFAsImageMedia(t *testing.T) {
	dir := t.TempDir()
	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()

	req := newUploadRequest(t, "animation.gif", []byte("gif-content"))
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
	if item.Type != "image" || !strings.HasSuffix(item.Filename, ".gif") {
		t.Fatalf("expected uploaded gif image item, got %#v", item)
	}
	if _, err := os.Stat(filepath.Join(dir, item.Filename)); err != nil {
		t.Fatalf("expected uploaded gif file: %v", err)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, "*.board")); err != nil || len(matches) != 0 {
		t.Fatalf("expected no board placeholder for uploaded gif, matches=%v err=%v", matches, err)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".boards", "*_bgimg.gif")); err != nil || len(matches) != 0 {
		t.Fatalf("expected no board background for uploaded gif, matches=%v err=%v", matches, err)
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
	if upload.Items[0].Filename != "clip.mp4" || upload.Items[1].Filename != "clip (1).mp4" {
		t.Fatalf("expected original filename plus numeric suffix, got %#v", upload.Items)
	}
	if upload.Items[0].DisplayName != "clip.mp4" || upload.Items[1].DisplayName != "clip (1).mp4" {
		t.Fatalf("expected display names to use unique filenames, got %#v", upload.Items)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/feed?index=-1", nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	var feedItem media.IndexedItem
	if err := json.NewDecoder(res.Body).Decode(&feedItem); err != nil {
		t.Fatal(err)
	}
	if feedItem.LastIndex != 1 {
		t.Fatalf("expected both uploaded files in feed, got %#v", feedItem)
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
	if board.ID == "" || board.MediaID == "" || board.MediaID != board.ID {
		t.Fatalf("expected board id to be the media id, got %#v", board)
	}
	if board.Filename != "Sketch.board" || board.Name != "Sketch" {
		t.Fatalf("expected board filename from submitted name, got %#v", board)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/media/"+board.MediaID+"/comments", bytes.NewBufferString(`{"text":"works","author":"Tester"}`))
	req.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected board comment to be accepted, got %d body=%s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/feed?index=-1", nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	var feedItem media.IndexedItem
	if err := json.NewDecoder(res.Body).Decode(&feedItem); err != nil {
		t.Fatal(err)
	}
	if feedItem.Item.ID != board.MediaID || feedItem.Item.CommentCount != 1 {
		t.Fatalf("expected indexed board media item with comment summary, got %#v", feedItem)
	}
}

func TestCreateStrokeReturnsNoContentAndPersistsStroke(t *testing.T) {
	dir := t.TempDir()
	handler := New(media.NewLibrary(dir), dir, "", log.New(io.Discard, "", 0)).Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/boards", bytes.NewBufferString(`{"name":"Sketch"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected board creation status 201, got %d body=%s", res.Code, res.Body.String())
	}

	var board media.BoardInfo
	if err := json.NewDecoder(res.Body).Decode(&board); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/boards/"+board.ID+"/strokes", bytes.NewBufferString(`{"tool":"freeform","points":[[1,2],[3,4]],"color":"#fff","size":4,"author":"Tester"}`))
	req.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected stroke creation status 204, got %d body=%s", res.Code, res.Body.String())
	}
	if res.Body.Len() != 0 {
		t.Fatalf("expected empty stroke creation body, got %q", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/boards/"+board.ID, nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected board fetch status 200, got %d body=%s", res.Code, res.Body.String())
	}

	var data struct {
		Strokes []media.Stroke `json:"strokes"`
	}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	if len(data.Strokes) != 1 || data.Strokes[0].Author != "Tester" {
		t.Fatalf("expected persisted stroke, got %#v", data.Strokes)
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
