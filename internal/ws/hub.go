package ws 

import (
	"context"
	"sync" 
	"RTChat/internal/pubsub"
	"RTChat/internal/storage"
	"RTChat/internal/ratelimit" 
)

type Hub struct {
	mu sync.Mutex 
	rooms map[string]*Room 
	store *storage.Store 
	limiter *ratelimit.Limiter
	bus pubsub.Broadcaster 

	ctx context.Context
	cancel context.CancelFunc
	wg sync.WaitGroup 
}

func NewHub(store *storage.Store, limiter *ratelimit.Limiter, bus pubsub.Broadcaster) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub {
		rooms: make(map[string]*Room), 
		store: store, 
		limiter: limiter, 
		bus: bus, 
		ctx: ctx, 
		cancel: cancel,
	}
}

func (h *Hub) GetOrCreateRoom(id string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[id]; ok {
		return r 
	}

	r := newRoom(id, h.store, h.limiter, h.bus) 
	h.rooms[id] = r 

	h.wg.Add(1)
	go func() {
		defer h.wg.Done() 
		r.run(h.ctx)
	}()  
	return r
}

func (h *Hub) Shutdown() {
	h.cancel() 
	h.mu.Lock() 
	for _, r := range h.rooms {
		close(r.done) 
	}
	h.mu.Unlock()  
	h.wg.Wait() 
}