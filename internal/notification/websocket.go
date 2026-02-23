package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/GooferByte/kalpi/internal/models"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Hub maintains the set of active WebSocket clients and broadcasts messages to them.
// Based on the canonical gorilla/websocket hub pattern.
type Hub struct {
	clients    map[*wsClient]bool
	broadcast  chan []byte
	register   chan *wsClient
	unregister chan *wsClient
	mu         sync.RWMutex
	logger     *zap.Logger
}

type wsClient struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for development; tighten in production.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// NewWebSocketHub creates a Hub. Call hub.Run() in a goroutine before using it.
func NewWebSocketHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[*wsClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient),
		logger:     logger,
	}
}

// Run is the hub's event loop. Must be called in its own goroutine.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
			h.logger.Info("ws client connected", zap.Int("total", len(h.clients)))

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
			h.logger.Info("ws client disconnected", zap.Int("total", len(h.clients)))

		case msg := <-h.broadcast:
			h.mu.Lock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					close(c.send)
					delete(h.clients, c)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Broadcast sends raw bytes to all connected clients.
func (h *Hub) Broadcast(data []byte) {
	select {
	case h.broadcast <- data:
	default:
		h.logger.Warn("ws broadcast channel full, dropping message")
	}
}

// ServeWS upgrades the HTTP connection to a WebSocket and registers the client.
// Call this from your HTTP handler.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ws upgrade failed", zap.Error(err))
		return
	}
	c := &wsClient{hub: h, conn: conn, send: make(chan []byte, 256)}
	h.register <- c

	go c.writePump()
	go c.readPump()
}

// readPump drains incoming messages (we treat the WS as write-only from the server).
func (c *wsClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

// writePump sends queued messages to the WebSocket connection.
func (c *wsClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// WebSocketNotifier implements notification.Notifier by broadcasting to all hub clients.
type WebSocketNotifier struct {
	hub    *Hub
	logger *zap.Logger
}

// NewWebSocketNotifier creates a Notifier that broadcasts to the given Hub.
func NewWebSocketNotifier(hub *Hub, logger *zap.Logger) Notifier {
	return &WebSocketNotifier{hub: hub, logger: logger}
}

func (n *WebSocketNotifier) Notify(_ context.Context, r *models.ExecutionResult) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	n.hub.Broadcast(data)
	return nil
}
