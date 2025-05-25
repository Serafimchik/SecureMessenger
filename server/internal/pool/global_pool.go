package pool

import (
	"SecureMessenger/server/internal/services"
	"context"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type ClientPool interface {
	AddClient(userID int, conn *websocket.Conn)
	GetClient(userID int) *Client
	RemoveClient(userID int)
	BroadcastEvent(chatID int, eventType string, data interface{})
}

type Client struct {
	UserID int
	Conn   *websocket.Conn
	Ctx    context.Context
	Cancel context.CancelFunc
}

var chatService services.ChatService

func init() {
	userService := services.NewUserService()
	chatService = services.NewChatService(userService)
}

type Pool struct {
	mu      sync.Mutex
	clients map[int]*Client
}

var GlobalPool ClientPool = &Pool{
	clients: make(map[int]*Client),
}

func (p *Pool) AddClient(userID int, conn *websocket.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	p.clients[userID] = &Client{
		UserID: userID,
		Conn:   conn,
		Ctx:    ctx,
		Cancel: cancel,
	}
	log.Printf("Client %d added to pool", userID)
}

func (p *Pool) GetClient(userID int) *Client {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.clients[userID]
}

func (p *Pool) RemoveClient(userID int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.clients, userID)
	log.Printf("Client %d removed from pool", userID)
}

func (p *Pool) BroadcastEvent(chatID int, eventType string, data interface{}) {
	participants, err := chatService.GetParticipantsByChatId(context.Background(), chatID)
	if err != nil {
		log.Printf("Error getting participants for chat %d: %v", chatID, err)
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, participant := range participants {
		client := p.clients[participant.ID]
		if client == nil {
			continue
		}

		err := client.Conn.WriteJSON(map[string]interface{}{
			"event": eventType,
			"data":  data,
		})
		if err != nil {
			log.Printf("Error sending event to user %d: %v", participant.ID, err)
			client.Conn.Close()
			p.RemoveClient(participant.ID)
		}

		log.Printf("sending event to user %d", participant.ID)
	}
}
