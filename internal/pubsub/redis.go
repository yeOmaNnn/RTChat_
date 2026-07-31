package pubsub

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Broadcaster interface {
	Publish(ctx context.Context, roomID string, payload []byte) error 
	Subscribe(ctx context.Context, roomID string, handler func(payload []byte))
	Close() error 
}

type NoOp struct{}

func NewNoOp() *NoOp {return &NoOp{} }

func (n *NoOp) Publish(ctx context.Context, roomID string, payload []byte) error { return nil }
func (n *NoOp) Subscribe(ctx context.Context, roomID string, handler func(payload []byte)) {}
func (n *NoOp) Close() error {return nil}

type RedisBroadcaster struct {
	client *redis.Client 
}

func NewRedis(addr string) (*RedisBroadcaster, error) {
	client := redis.NewClient(&redis.Options{Addr : addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err 
	}
	return &RedisBroadcaster{client: client}, nil 
}

func (r *RedisBroadcaster) channelName(roomID string) string {
	return "chat:room:" + roomID
}

func (r *RedisBroadcaster) Publish(ctx context.Context, roomID string, payload []byte) error {
	return r.client.Publish(ctx, r.channelName(roomID), payload).Err()
}

func (r *RedisBroadcaster) Subscribe(ctx context.Context, roomID string, handler func(payload []byte)) {
	sub := r.client.Subscribe(ctx, r.channelName(roomID))
	ch := sub.Channel()
	go func() {
		defer sub.Close()
		for { select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			handler([]byte(msg.Payload))
		}}
	}()
}

func (r *RedisBroadcaster) Close() error {
	return r.client.Close()
}
