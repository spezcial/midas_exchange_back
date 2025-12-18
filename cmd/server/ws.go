package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/logger"
	"github.com/gorilla/websocket"
	"net/http"
)

type WebSocketService struct {
	upgrader             *websocket.Upgrader
	logger               *logger.Logger
	exchangeRatesService *service.ExchangeRatesService
}

type WsRequest struct {
	Action string `json:"action"`
}

type GetExchangeRateRequest struct {
	From int32 `json:"from"`
	To   int32 `json:"to"`
}

func NewWebSocketService(exchangeRatesService *service.ExchangeRatesService, logger *logger.Logger, allowedOrigins string, readBufferSize, writeBufferSize int) *WebSocketService {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  readBufferSize,
		WriteBufferSize: writeBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			if allowedOrigins == "*" {
				return true
			}
			return true // Allow all origins for simplicity, adjust for production
		},
	}

	return &WebSocketService{exchangeRatesService: exchangeRatesService, logger: logger, upgrader: &upgrader}
}

func (ws *WebSocketService) handler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.logger.Error("upgrader.Upgrade:", err)
		http.Error(w, "Could not upgrade to WebSocket", http.StatusInternalServerError)
		cancel()
		return
	}

	defer cancel()
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			ws.logger.Error("Error reading from websocket", err)
			break
		}

		response, err := ws.processMessage(ctx, message)
		if err != nil {
			ws.logger.Error("Socket error", err)
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, response); err != nil {
			ws.logger.Error("Error writing to websocket", err)
			break
		}
	}
}

func (ws *WebSocketService) processMessage(ctx context.Context, message []byte) ([]byte, error) {
	var request WsRequest
	err := json.Unmarshal(message, &request)
	if err != nil {
		return []byte{}, err
	}

	ws.logger.Info("Received message from websocket. Action: ", request.Action)

	switch request.Action {
	case "ping":
		return []byte("pong"), nil
	case "get_exchange_rate":
		var exchangeReq GetExchangeRateRequest
		err := json.Unmarshal(message, &exchangeReq)
		if err != nil {
			return []byte{}, err
		}
		rate, err := ws.exchangeRatesService.GetRateByPair(ctx, exchangeReq.From, exchangeReq.To)
		if err != nil {
			return []byte{}, err
		}
		return json.Marshal(rate)
	}

	return []byte{}, errors.New("unknown message")
}
