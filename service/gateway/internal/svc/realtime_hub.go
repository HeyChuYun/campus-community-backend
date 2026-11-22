package svc

import "sync"

type RealtimeClient interface {
	SendJSON(payload map[string]any) bool
}

type RealtimeHub struct {
	mu      sync.RWMutex
	clients map[int64]map[RealtimeClient]struct{}
}

func NewRealtimeHub() *RealtimeHub {
	return &RealtimeHub{
		clients: make(map[int64]map[RealtimeClient]struct{}),
	}
}

func (h *RealtimeHub) Register(userID int64, client RealtimeClient) {
	if h == nil || userID <= 0 || client == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[RealtimeClient]struct{})
	}
	h.clients[userID][client] = struct{}{}
}

func (h *RealtimeHub) Unregister(userID int64, client RealtimeClient) {
	if h == nil || userID <= 0 || client == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	clients := h.clients[userID]
	if clients == nil {
		return
	}
	delete(clients, client)
	if len(clients) == 0 {
		delete(h.clients, userID)
	}
}

func (h *RealtimeHub) Broadcast(userID int64, payload map[string]any) {
	if h == nil || userID <= 0 || payload == nil {
		return
	}

	h.mu.RLock()
	clients := make([]RealtimeClient, 0, len(h.clients[userID]))
	for client := range h.clients[userID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		if ok := client.SendJSON(payload); !ok {
			h.Unregister(userID, client)
		}
	}
}
