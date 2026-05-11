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
	mu                sync.Mutex
	subscribers       map[string]map[chan feedEvent]struct{}
	globalSubscribers map[chan feedEvent]struct{}
}

func newBoardHub() *boardHub {
	return &boardHub{
		subscribers:       make(map[string]map[chan feedEvent]struct{}),
		globalSubscribers: make(map[chan feedEvent]struct{}),
	}
}

func (h *boardHub) subscribeAll() chan feedEvent {
	ch := make(chan feedEvent, 64)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.globalSubscribers[ch] = struct{}{}
	return ch
}

func (h *boardHub) unsubscribeAll(ch chan feedEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.globalSubscribers, ch)
	close(ch)
}

func (h *boardHub) subscribe(boardID string) chan feedEvent {
	ch := make(chan feedEvent, 16)

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.subscribers[boardID] == nil {
		h.subscribers[boardID] = make(map[chan feedEvent]struct{})
	}
	h.subscribers[boardID][ch] = struct{}{}

	return ch
}

func (h *boardHub) unsubscribe(boardID string, ch chan feedEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subs, ok := h.subscribers[boardID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(h.subscribers, boardID)
		}
	}
	close(ch)
}

func (h *boardHub) publishStroke(boardID string, stroke media.Stroke) {
	event := feedEvent{
		Name: "stroke",
		Data: strokeEvent{BoardID: boardID, Stroke: stroke},
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Direct subscribers
	for ch := range h.subscribers[boardID] {
		select {
		case ch <- event:
		default:
		}
	}

	// Global subscribers
	for ch := range h.globalSubscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
