// Package service — SSE (Server-Sent Events) 事件流服务。
package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SSEHub 管理 SSE 客户端连接和事件广播。
type SSEHub struct {
	clients    map[chan SSEEvent]bool
	broadcast  chan SSEEvent
	register   chan chan SSEEvent
	unregister chan chan SSEEvent
	log        *zap.Logger
	tickets    map[string]*sseTicket
	ticketMu   sync.RWMutex
	stopCh     chan struct{}
}

type SSEEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// SSEEvent 事件类型常量。
const (
	EventTypeScan      = "scan"
	EventTypeDownload  = "download"
	EventTypeSubscribe = "subscribe"
	EventTypeTask      = "task"
	EventTypeSystem    = "system"
	EventTypeAuth      = "auth"
)

// sseTicket 是一次性 OTP 票据。
type sseTicket struct {
	UserID    string
	ExpiresAt time.Time
}

// NewSSEHub 创建 SSE Hub 实例。
func NewSSEHub(log *zap.Logger) *SSEHub {
	return &SSEHub{
		clients:    make(map[chan SSEEvent]bool),
		broadcast:  make(chan SSEEvent, 256),
		register:   make(chan chan SSEEvent),
		unregister: make(chan chan SSEEvent),
		tickets:    make(map[string]*sseTicket),
		log:        log,
		stopCh:     make(chan struct{}),
	}
}

// Run 启动 SSE Hub 的事件循环。
func (h *SSEHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			h.log.Debug("SSE client connected", zap.Int("total", len(h.clients)))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client)
				h.log.Debug("SSE client disconnected", zap.Int("total", len(h.clients)))
			}

		case event := <-h.broadcast:
			h.distribute(event)

		case <-h.stopCh:
			h.closeAll()
			return
		}
	}
}

// Stop 停止 SSE Hub。
func (h *SSEHub) Stop() {
	close(h.stopCh)
}

// ClientChannel SSE 客户端通道包装器。
type ClientChannel struct {
	Ch chan SSEEvent
}

// Subscribe 注册一个新的 SSE 客户端，返回 SSE 客户端包装器。
func (h *SSEHub) Subscribe() *ClientChannel {
	ch := make(chan SSEEvent, 100)
	h.register <- ch
	return &ClientChannel{Ch: ch}
}

// Unsubscribe 取消注册 SSE 客户端。
func (h *SSEHub) Unsubscribe(client *ClientChannel) {
	if client != nil && client.Ch != nil {
		h.unregister <- client.Ch
	}
}

// Broadcast 向所有连接的客户端广播事件。
func (h *SSEHub) Broadcast(eventType string, payload interface{}) {
	event := SSEEvent{
		Type:    eventType,
		Payload: payload,
	}
	select {
	case h.broadcast <- event:
	default:
		h.log.Warn("SSE broadcast queue full, dropping event", zap.String("type", eventType))
	}
}

// SendToUser 向指定用户发送事件（通过 UserID 匹配）。
// 注意：此方法需要在客户端连接时关联 UserID。
func (h *SSEHub) SendToUser(userID string, eventType string, payload interface{}) {
	// 目前通过广播实现，未来可扩展为按用户分组
	h.Broadcast(eventType, payload)
}

// distribute 将事件分发给所有客户端。
func (h *SSEHub) distribute(event SSEEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		h.log.Error("failed to marshal SSE event", zap.Error(err))
		return
	}

	for client := range h.clients {
		select {
		case client <- event:
		default:
			// 客户端通道已满，跳过
			h.log.Warn("SSE client buffer full", zap.String("event", string(data)))
		}
	}
}

// closeAll 关闭所有客户端连接。
func (h *SSEHub) closeAll() {
	for client := range h.clients {
		close(client)
	}
	h.clients = make(map[chan SSEEvent]bool)
}

// GenerateTicket 生成一次性 SSE 连接票据（用于无 JWT 场景下的安全连接）。
func (h *SSEHub) GenerateTicket(userID string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	ticket := hex.EncodeToString(buf)

	h.ticketMu.Lock()
	defer h.ticketMu.Unlock()

	h.tickets[ticket] = &sseTicket{
		UserID:    userID,
		ExpiresAt: time.Now().Add(10 * time.Second),
	}

	return ticket, nil
}

// ValidateTicket 验证 SSE 连接票据，返回关联的用户 ID。
func (h *SSEHub) ValidateTicket(ticket string) (string, error) {
	h.ticketMu.Lock()
	defer h.ticketMu.Unlock()

	t, ok := h.tickets[ticket]
	if !ok {
		return "", ErrInvalidTicket
	}

	if time.Now().After(t.ExpiresAt) {
		delete(h.tickets, ticket)
		return "", ErrTicketExpired
	}

	userID := t.UserID
	delete(h.tickets, ticket)

	return userID, nil
}

// CleanupTickets 清理过期的票据。
func (h *SSEHub) CleanupTickets() {
	h.ticketMu.Lock()
	defer h.ticketMu.Unlock()

	now := time.Now()
	for ticket, t := range h.tickets {
		if now.After(t.ExpiresAt) {
			delete(h.tickets, ticket)
		}
	}
}

// SSE Hub 错误定义。
var (
	ErrInvalidTicket = &SSEError{Message: "invalid ticket"}
	ErrTicketExpired = &SSEError{Message: "ticket expired"}
)

// SSEError SSE 相关错误。
type SSEError struct {
	Message string
}

func (e *SSEError) Error() string {
	return e.Message
}

// ClientCount 返回当前连接的客户端数量。
func (h *SSEHub) ClientCount() int {
	h.ticketMu.RLock()
	defer h.ticketMu.RUnlock()
	return len(h.clients)
}
