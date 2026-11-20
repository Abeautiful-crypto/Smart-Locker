package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"smart-locker/internal/store"
)

// wsHub 维护在线连接：key 为 "u:<userID>" 或 "a:<token>"。
// 用户发送的消息实时推给所有后台客服连接；客服回复实时推给目标用户。
type wsHub struct {
	mu    sync.RWMutex
	conns map[string]map[*websocket.Conn]bool
}

var chatHub = &wsHub{conns: map[string]map[*websocket.Conn]bool{}}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // 演示环境允许同源任意端口
}

func (h *wsHub) add(key string, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[key] == nil {
		h.conns[key] = map[*websocket.Conn]bool{}
	}
	h.conns[key][c] = true
}

func (h *wsHub) remove(key string, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.conns[key]; set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.conns, key)
		}
	}
}

func (h *wsHub) broadcast(key string, payload any) {
	h.mu.RLock()
	targets := make([]*websocket.Conn, 0, len(h.conns[key]))
	for c := range h.conns[key] {
		targets = append(targets, c)
	}
	h.mu.RUnlock()
	data, _ := json.Marshal(payload)
	for _, c := range targets {
		_ = c.SetWriteDeadline(time.Now().Add(3 * time.Second))
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			h.remove(key, c)
			_ = c.Close()
		}
	}
}

// broadcastAdmins 推送给全部后台客服连接
func (h *wsHub) broadcastAdmins(payload any) {
	h.mu.RLock()
	keys := make([]string, 0, len(h.conns))
	for k := range h.conns {
		if len(k) > 0 && k[0] == 'a' {
			keys = append(keys, k)
		}
	}
	h.mu.RUnlock()
	for _, k := range keys {
		h.broadcast(k, payload)
	}
}

// handleUserWS 用户端 WebSocket：/api/ws/chat?token=<用户token>
func (s *Server) handleUserWS(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authUser(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	c, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] 用户升级失败: %v", err)
		return
	}
	key := "u:" + itoa(claims.refID)
	chatHub.add(key, c)
	defer func() { chatHub.remove(key, c); _ = c.Close() }()
	// 用户连接建立后通知后台刷新会话列表
	chatHub.broadcastAdmins(map[string]any{"type": "sessions_changed"})
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			return
		}
		var msg struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		}
		if json.Unmarshal(data, &msg) != nil || msg.Type != "chat" {
			continue
		}
		// 复用 HTTP 路径的业务逻辑：直接写库
		_ = s.st.AppendMessage(r.Context(), &store.ChatMessage{UserID: claims.refID, Sender: "user", Content: msg.Content, Channel: "websocket"})
		chatHub.broadcast(key, map[string]any{"type": "message", "sender": "user", "content": msg.Content})
		chatHub.broadcastAdmins(map[string]any{"type": "sessions_changed"})
	}
}

// handleAdminWS 后台 WebSocket：/api/admin/ws/chat?token=<后台token>
func (s *Server) handleAdminWS(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	c, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] 后台升级失败: %v", err)
		return
	}
	key := "a:" + claims.Token
	chatHub.add(key, c)
	defer func() { chatHub.remove(key, c); _ = c.Close() }()
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			return
		}
	}
}

// notifyUserMessage / notifyAgentMessage 供 HTTP handler 在写入消息后调用，
// 保证纯 HTTP 路径（WebSocket 不可用时）也能把消息实时推给在线端。
func notifyUserMessage(userID int64, sender, content string) {
	chatHub.broadcast(itoaKey("u:", userID), map[string]any{"type": "message", "sender": sender, "content": content})
	chatHub.broadcastAdmins(map[string]any{"type": "sessions_changed"})
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [24]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func itoaKey(prefix string, v int64) string { return prefix + itoa(v) }
