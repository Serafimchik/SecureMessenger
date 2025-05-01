package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"

	"SecureMessenger/server/internal/pool"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte("secret-key"), nil
	})

	if err != nil || !token.Valid {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["user_id"] == nil || claims["username"] == nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	userID := int(claims["user_id"].(float64))
	username := claims["username"].(string)

	log.Printf("Authenticated user ID: %d, Username: %s", userID, username)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("User %d connected to WebSocket", userID)

	clientPool := pool.GlobalPool
	clientPool.AddClient(userID, conn)

	for {
		var msg struct {
			Event   string `json:"event"`
			ChatID  int    `json:"chat_id"`
			Content string `json:"content"`
		}

		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("Error reading message from user %d: %v", userID, err)
			break
		}

		log.Printf("User %d sent event '%s' to chat %d: %s", userID, msg.Event, msg.ChatID, msg.Content)

		switch msg.Event {
		case "send_message":
			sentAt, err := chatService.SaveMessage(r.Context(), msg.ChatID, userID, username, msg.Content)
			if err != nil {
				log.Printf("Error saving message: %v", err)
				continue
			}

			eventData := map[string]interface{}{
				"sender_id": strconv.Itoa(userID),
				"username":  username,
				"content":   msg.Content,
				"chat_id":   msg.ChatID,
				"sent_at":   sentAt.Format(time.RFC3339),
			}

			clientPool.BroadcastEvent(msg.ChatID, "new_message", eventData)

			log.Printf("Message sent to chat %d by user %d (%s) at %s", msg.ChatID, userID, username, sentAt)

		case "create_chat":
			var createChatReq struct {
				RecipientEmail string  `json:"recipient_email"`
				Type           string  `json:"type"`
				Name           *string `json:"name"`
			}
			err := json.Unmarshal([]byte(msg.Content), &createChatReq)
			if err != nil || createChatReq.RecipientEmail == "" || createChatReq.Type == "" {
				log.Printf("Invalid create_chat request from user %d: %v", userID, err)
				continue
			}

			recipient, err := userService.GetUserByEmail(r.Context(), createChatReq.RecipientEmail)
			if err != nil {
				log.Printf("Error getting user by email %s: %v", createChatReq.RecipientEmail, err)
				continue
			}

			chatID, err := chatService.CreateChat(r.Context(), userID, recipient.ID, createChatReq.Type, createChatReq.Name)
			if err != nil {
				log.Printf("Error creating chat between user %d and recipient %d: %v", userID, recipient.ID, err)
				continue
			}

			clientPool.BroadcastEvent(chatID, "new_chat", map[string]int{
				"chat_id": chatID,
			})

			log.Printf("Chat created with ID %d between user %d and recipient %d", chatID, userID, recipient.ID)
		}
	}
}
