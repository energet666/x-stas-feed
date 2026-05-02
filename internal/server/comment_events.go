package server

import (
	"sync"

	"feed-ai/internal/media"
)

type commentEvent struct {
	MediaID string        `json:"mediaId"`
	Comment media.Comment `json:"comment"`
}

type commentHub struct {
	mu          sync.Mutex
	subscribers map[chan commentEvent]struct{}
}

func newCommentHub() *commentHub {
	return &commentHub{subscribers: make(map[chan commentEvent]struct{})}
}

func (h *commentHub) subscribe() chan commentEvent {
	ch := make(chan commentEvent, 16)

	h.mu.Lock()
	defer h.mu.Unlock()

	h.subscribers[ch] = struct{}{}

	return ch
}

func (h *commentHub) unsubscribe(ch chan commentEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.subscribers, ch)
	close(ch)
}

func (h *commentHub) publish(mediaID string, comment media.Comment) {
	h.mu.Lock()
	defer h.mu.Unlock()

	event := commentEvent{MediaID: mediaID, Comment: comment}
	for ch := range h.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
