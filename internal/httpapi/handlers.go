package httpapi

import (
	"RTChat/internal/ws"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"RTChat/internal/storage"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader {
	ReadBufferSize: 1024, 
	WriteBufferSize: 1024, 
	CheckOrigin: func (r *http.Request) bool {return true }, 
}

type Server struct {
	hub *ws.Hub 
	store *storage.Store
	historyLimit int 
}

func NewServer(hub *ws.Hub, store *storage.Store, historyLimit int) *Server {
	return &Server{hub: hub, store: store, historyLimit: historyLimit } 
}

func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/rooms/", s.handleHistory)
	mux.HandleFunc("/ws", s.handleWS)
	return mux 
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	username := r.URL.Query().Get("username")
	if roomID == "" || username == "" {
		http.Error(w, "нужны параметры ?room= и ?username=", http.StatusBadRequest)
		return 
	}

	conn, err := upgrader.Upgrade(w, r, nil) 
	if err != nil {
		log.Printf("Upgrader выдал ошибку: %v", err)
		return 
	}

	room := s.hub.GetOrCreateRoom(roomID)
	clientID := uuid.NewString()
	client := ws.NewClient(clientID, username, roomID, conn, room)

	room.Register(client)

	go s.sendHistory(client, roomID)
	go client.WritePump()
	go client.ReadPump(r.Context()) 
}

func (s *Server) sendHistory(c *ws.Client, roomID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := s.store.History(ctx, roomID, s.historyLimit)
	if err != nil {
		log.Printf("не удалось загрузить историю сообщении: %s, %v", roomID, err)
		return
	}

	payload, _ := json.Marshal(map[string]any{"history": msgs})
	c.SendRaw(payload)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	roomID := extractRoomID(r.URL.Path) 
	if roomID == "" {
		http.Error(w, "не найден айдишка комнаты", http.StatusBadRequest)
		return
	}

	msgs, err := s.store.History(r.Context(), roomID, s.historyLimit) 
	if err != nil {
		http.Error(w, "ошибка загрузки истории", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs) 
}

func extractRoomID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "api" || parts[1] != "rooms" {
		return ""
	}

	return parts[2]
}