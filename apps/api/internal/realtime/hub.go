package realtime

import "sync"

type Hub struct {
	mu          sync.RWMutex
	connections int
}

func NewHub() *Hub {
	return &Hub{}
}

func (h *Hub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connections
}
