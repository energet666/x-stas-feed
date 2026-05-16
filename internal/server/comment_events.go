package server

import (
	"sync"

	"feed-ai/internal/media"
)

type commentEvent struct {
	MediaID string        `json:"mediaId"`
	Comment media.Comment `json:"comment"`
}

type likeEvent struct {
	MediaID   string `json:"mediaId"`
	LikeCount int    `json:"likeCount"`
}

type commentLikeEvent struct {
	MediaID   string `json:"mediaId"`
	CommentID string `json:"commentId"`
	LikeCount int    `json:"likeCount"`
}

type feedItemCreatedEvent struct {
	Index      int        `json:"index"`
	FirstIndex int        `json:"firstIndex"`
	LastIndex  int        `json:"lastIndex"`
	Item       media.Item `json:"item"`
}

type feedEvent struct {
	Name string
	Data any
}

type commentHub struct {
	mu          sync.Mutex
	subscribers map[chan feedEvent]struct{}
}

func newCommentHub() *commentHub {
	return &commentHub{subscribers: make(map[chan feedEvent]struct{})}
}

func (h *commentHub) subscribe() chan feedEvent {
	ch := make(chan feedEvent, 16)

	h.mu.Lock()
	defer h.mu.Unlock()

	h.subscribers[ch] = struct{}{}

	return ch
}

func (h *commentHub) unsubscribe(ch chan feedEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.subscribers, ch)
	close(ch)
}

func (h *commentHub) publish(mediaID string, comment media.Comment) {
	h.publishEvent(feedEvent{
		Name: "comment",
		Data: commentEvent{MediaID: mediaID, Comment: comment},
	})
}

func (h *commentHub) publishLike(mediaID string, likeCount int) {
	h.publishEvent(feedEvent{
		Name: "like",
		Data: likeEvent{MediaID: mediaID, LikeCount: likeCount},
	})
}

func (h *commentHub) publishCommentLike(mediaID, commentID string, likeCount int) {
	h.publishEvent(feedEvent{
		Name: "comment-like",
		Data: commentLikeEvent{MediaID: mediaID, CommentID: commentID, LikeCount: likeCount},
	})
}

func (h *commentHub) publishFeedItemCreated(item media.IndexedItem) {
	h.publishEvent(feedEvent{
		Name: "feed-item-created",
		Data: feedItemCreatedEvent{
			Index:      item.Index,
			FirstIndex: item.FirstIndex,
			LastIndex:  item.LastIndex,
			Item:       item.Item,
		},
	})
}

func (h *commentHub) publishEvent(event feedEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
