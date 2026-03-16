package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type dashboardEvent struct {
	Type          string `json:"type"`
	EventID       uint64 `json:"event_id"`
	ActorClientID string `json:"actor_client_id,omitempty"`
	At            string `json:"at"`
}

type dashboardEventHub struct {
	mu               sync.RWMutex
	subscribers      map[string]map[uint64]chan dashboardEvent
	nextSubscriberID uint64
	nextEventID      uint64
}

func newDashboardEventHub() *dashboardEventHub {
	return &dashboardEventHub{
		subscribers: make(map[string]map[uint64]chan dashboardEvent),
	}
}

func (h *dashboardEventHub) Subscribe(userID string) (uint64, <-chan dashboardEvent, func()) {
	subscriberID := atomic.AddUint64(&h.nextSubscriberID, 1)
	stream := make(chan dashboardEvent, 8)

	h.mu.Lock()
	if _, ok := h.subscribers[userID]; !ok {
		h.subscribers[userID] = make(map[uint64]chan dashboardEvent)
	}
	h.subscribers[userID][subscriberID] = stream
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		userSubscribers, ok := h.subscribers[userID]
		if !ok {
			return
		}

		if subscriber, ok := userSubscribers[subscriberID]; ok {
			delete(userSubscribers, subscriberID)
			close(subscriber)
		}

		if len(userSubscribers) == 0 {
			delete(h.subscribers, userID)
		}
	}

	return subscriberID, stream, cancel
}

func (h *dashboardEventHub) Publish(userID string, actorClientID string) {
	event := dashboardEvent{
		Type:          "dashboard-updated",
		EventID:       atomic.AddUint64(&h.nextEventID, 1),
		ActorClientID: actorClientID,
		At:            time.Now().UTC().Format(time.RFC3339Nano),
	}

	h.mu.RLock()
	userSubscribers := h.subscribers[userID]
	streams := make([]chan dashboardEvent, 0, len(userSubscribers))
	for _, stream := range userSubscribers {
		streams = append(streams, stream)
	}
	h.mu.RUnlock()

	for _, stream := range streams {
		select {
		case stream <- event:
		default:
		}
	}
}

func (h *Handler) publishDashboardUpdate(userID, actorClientID string) {
	if h.eventHub == nil || strings.TrimSpace(userID) == "" {
		return
	}
	h.eventHub.Publish(userID, strings.TrimSpace(actorClientID))
}

func requestClientID(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Todo-Client-ID"))
}

func (h *Handler) handleEventStream(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		http.Error(w, "请先登录", http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	_, stream, cancel := h.eventHub.Subscribe(user.ID.String())
	defer cancel()

	fmt.Fprint(w, "retry: 2000\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		case event, ok := <-stream:
			if !ok {
				return
			}

			payload, err := json.Marshal(event)
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "id: %d\n", event.EventID)
			fmt.Fprintf(w, "event: dashboard\n")
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}
