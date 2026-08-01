package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait = 10 * time.Second 
	pongWait = 60 * time.Second 
	pingPeriod = (pongWait * 9) / 10
	maxMessageSize = 4096
) 

type IncomingMessage struct {
	Content string `json:"content"`
}

type OutgoingMessage struct {
	RoomID string `string:"room_id"`
	Username string `string:"username"`
	Content string `string:"content"`
	CreatedAt time.Time `string:"created_at"`
}

type Client struct {
	ID string 
	Username string 
	RoomID string 
	conn *websocket.Conn
	room *Room 
	send chan []byte
}

func NewClient(id, username, roomID string, conn *websocket.Conn, room *Room) *Client {
	return &Client{
		ID: id,
		Username: username, 
		RoomID: roomID,
		conn: conn, 
		room: room,
		send: make(chan []byte, 32),
}
}

func (c *Client) SendRaw(payload []byte) {
	select {
	case c.send <- payload:
	default:
	}
}

func (c *Client) sendSystem(text string) {
	payload, _ := json.Marshal(map[string]string{"system": text}) 
	c.SendRaw(payload)
}

func (c *Client) ReadPump(ctx context.Context) {
	defer func() {
		c.room.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize) 
	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
	_, raw, err := c.conn.ReadMessage()
	if err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			log.Printf("Клиент %s закрыт: %v", c.ID, err)
	}
	return 
	}

	raw = bytes.TrimSpace(raw)
	var in IncomingMessage 
	if err := json.Unmarshal(raw, &in); err != nil  || in.Content == "" { 
		continue
	}

	if !c.room.limiter.Allow(c.ID) {
		c.sendSystem("Слишком много пишешь, зачиль!")
		continue
	}

	out := OutgoingMessage{
		RoomID: c.RoomID, 
		Username: c.Username, 
		Content: in.Content, 
		CreatedAt: time.Now(),
	}
	c.room.incoming <- clientMessage{from: c, msg: out}
}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod) 
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return 
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return 
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return 
			}
		}
	}
}

