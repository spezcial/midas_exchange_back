package main

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/caspianex/exchange-backend/pkg/auth"
	"github.com/caspianex/exchange-backend/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// OTCHub manages WebSocket connections grouped by OTC order UID.
// Connections join a room on upgrade and leave on disconnect.
// HTTP handlers call Broadcast after each successful message/offer to push
// the new message to every connected participant of that order.
type OTCHub struct {
	mu         sync.RWMutex
	rooms      map[string]map[*websocket.Conn]struct{}
	upgrader   websocket.Upgrader
	jwtManager *auth.JWTManager
	log        *logger.Logger
}

func NewOTCHub(jwtManager *auth.JWTManager, log *logger.Logger) *OTCHub {
	return &OTCHub{
		rooms:      make(map[string]map[*websocket.Conn]struct{}),
		jwtManager: jwtManager,
		log:        log,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// Broadcast sends msg as JSON to every WebSocket connection in the order's room.
// Implements service.OTCBroadcaster.
func (h *OTCHub) Broadcast(orderUID string, msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.log.Error("OTCHub: marshal error", "error", err)
		return
	}

	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.rooms[orderUID]))
	for c := range h.rooms[orderUID] {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	for _, c := range conns {
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			h.log.Error("OTCHub: write error", "error", err)
			h.leave(orderUID, c)
		}
	}
}

func (h *OTCHub) join(orderUID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[orderUID] == nil {
		h.rooms[orderUID] = make(map[*websocket.Conn]struct{})
	}
	h.rooms[orderUID][conn] = struct{}{}
}

func (h *OTCHub) leave(orderUID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if room := h.rooms[orderUID]; room != nil {
		delete(room, conn)
		if len(room) == 0 {
			delete(h.rooms, orderUID)
		}
	}
}

// ServeOTCChat upgrades to WebSocket for an OTC order chat room.
// Auth: ?token=<jwt> query parameter (Bearer header is unavailable during WS upgrade).
// The connection is server-push only — incoming frames are drained to detect disconnects.
func (h *OTCHub) ServeOTCChat(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	claims, err := h.jwtManager.ValidateToken(token)
	if err != nil || claims.Type != "access" {
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("OTCHub: upgrade error", "error", err)
		return
	}
	defer conn.Close()

	h.join(uid, conn)
	defer h.leave(uid, conn)

	h.log.Info("OTC WebSocket connected", "order_uid", uid, "user_id", claims.UserID)

	// Drain client frames; the server only pushes, never reads messages.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
