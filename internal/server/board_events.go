package server

import (
	"sync"

	"feed-ai/internal/media"
)

type strokeEvent struct {
	BoardID string       `json:"boardId"`
	Stroke  media.Stroke `json:"stroke"`
}

type boardHub struct {
	mu          sync.Mutex
	subscribers map[chan feedEvent]struct{}
}

func newBoardHub() *boardHub {
	return &boardHub{subscribers: make(map[chan feedEvent]struct{})}
}

func (h *boardHub) subscribeAll() chan feedEvent {
	ch := make(chan feedEvent, 64)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subscribers[ch] = struct{}{}
	return ch
}

func (h *boardHub) unsubscribeAll(ch chan feedEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subscribers, ch)
	close(ch)
}

func (h *boardHub) publishStroke(boardID string, stroke media.Stroke) {
	event := feedEvent{
		Name: "stroke",
		Data: strokeEvent{BoardID: boardID, Stroke: stroke},
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
