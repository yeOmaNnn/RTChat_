package ws

import (
	"RTChat/internal/pubsub"
	"RTChat/internal/ratelimit"
	"RTChat/internal/storage"
	"context"
	"encoding/json"
	"log"
)

type clientMessage struct {
	from *Client
	msg  OutgoingMessage
}

type Room struct {
	ID         string
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	incoming   chan clientMessage
	done       chan *Client

	store   *storage.Store
	limiter *ratelimit.Limiter
	bus     pubsub.Broadcaster
}

func (r *Room) Register(c *Client) {
	r.register <- c
}

func newRoom(id string, store *storage.Store, limiter *ratelimit.Limiter, bus pubsub.Broadcaster) *Room {
	return &Room{
		ID:         id,
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		incoming:   make(chan clientMessage),
		done:       make(chan *Client),
		store:      store,
		limiter:    limiter,
		bus:        bus,
	}
}

func (r *Room) run(ctx context.Context) {
	r.bus.Subscribe(ctx, r.ID, func(payload []byte) {
		r.broadcastRaw(payload, nil)
	})

	for {
		select {
		case c := <-r.register:
			r.clients[c] = true
			log.Printf("Комната %s: клиент %s подключен, всего клиентов: %d", r.ID, c.ID, c.Username,
				len(r.clients))

		case c := <-r.unregister:
			if _, ok := r.clients[c]; ok {
				delete(r.clients, c)
				close(c.send)
				r.limiter.Remove(c.ID)
				log.Printf("Комната %s: клиент %s отключился, осталось: %d", r.ID, c.ID, len(r.clients))
			}

		case cm := <-r.incoming:
			r.handleIncoming(ctx, cm)

		case <-r.done:
			for c := range r.clients {
				close(c.send)
				delete(r.clients, c)
			}

			return

		case <-ctx.Done():
			return
		}
	}
}

func (r *Room) handleIncoming(ctx context.Context, cm clientMessage) {
	saved, err := r.store.SaveMessage(ctx, storage.Message{
		RoomID:    cm.msg.RoomID,
		Username:  cm.msg.Username,
		Content:   cm.msg.Content,
		CreatedAt: cm.msg.CreatedAt,
	})

	if err != nil {
		log.Printf("Ошибка сохранения сообщения: %v", err)
		return
	}

	out := OutgoingMessage{
		RoomID:    saved.RoomID,
		Username:  saved.Username,
		Content:   saved.Content,
		CreatedAt: saved.CreatedAt,
	}

	payload, err := json.Marshal(out)
	if err != nil {
		log.Printf("Ошибка маршалинга сообщения: %v", err)
		return
	}

	r.broadcastRaw(payload, nil)
	if err := r.bus.Publish(ctx, r.ID, payload); err != nil {
		log.Printf("Ошибка публикации сообщения в Redis: %v", err)
	}
}

func (r *Room) broadcastRaw(payload []byte, exclude *Client) {
	for c := range r.clients {
		if c == exclude {
			continue
		}
		select {
		case c.send <- payload:
		default:
			close(c.send)
			delete(r.clients, c)
		}
	}
}
