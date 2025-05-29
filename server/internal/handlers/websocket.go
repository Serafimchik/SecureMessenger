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

	"SecureMessenger/server/internal/models"
	"SecureMessenger/server/internal/pool"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func broadcastToChat(chatID int, eventType string, eventData map[string]interface{}) {
	pool.GlobalPool.BroadcastEvent(chatID, eventType, eventData)
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
			messageID, sentAt, err := chatService.SaveMessage(r.Context(), msg.ChatID, userID, username, msg.Content)
			if err != nil {
				log.Printf("Error saving message: %v", err)
				continue
			}

			eventData := map[string]interface{}{
				"message_id": strconv.Itoa(messageID),
				"sender_id":  strconv.Itoa(userID),
				"username":   username,
				"content":    msg.Content,
				"chat_id":    msg.ChatID,
				"sent_at":    sentAt.Format(time.RFC3339),
			}

			clientPool.BroadcastEvent(msg.ChatID, "new_message", eventData)

			log.Printf("Message sent to chat %d by user %d (%s) at %s, Message ID: %d", msg.ChatID, userID, username, sentAt, messageID)

		case "create_chat":
			var createChatReq struct {
				RecipientEmail *string               `json:"recipient_email"`
				Type           string                `json:"type"`
				Name           *string               `json:"name"`
				Emails         []string              `json:"emails"`
				EncryptedKeys  []models.EncryptedKey `json:"encrypted_keys"`
				RawAESKey      *string               `json:"raw_aes_key"`
			}
			err := json.Unmarshal([]byte(msg.Content), &createChatReq)
			if err != nil || createChatReq.Type == "" {
				log.Printf("Invalid create_chat request from user %d: %v", userID, err)
				continue
			}

			switch createChatReq.Type {
			case "direct":
				if createChatReq.RecipientEmail == nil || *createChatReq.RecipientEmail == "" {
					log.Printf("Invalid create_chat request from user %d: recipient_email is required for direct chat", userID)
					continue
				}

				recipient, err := userService.GetUserByEmail(r.Context(), *createChatReq.RecipientEmail)
				if err != nil {
					log.Printf("Error getting user by email %s: %v", *createChatReq.RecipientEmail, err)
					continue
				}

				existingChatID, err := chatService.CheckExistingPrivateChat(r.Context(), userID, recipient.ID)
				if err != nil {
					log.Printf("Error checking existing private chat: %v", err)
					continue
				}

				if existingChatID > 0 {
					clientPool.BroadcastEvent(existingChatID, "new_chat", map[string]int{
						"chat_id": existingChatID,
					})
					log.Printf("Existing private chat found with ID %d", existingChatID)
					continue
				}

				chatID, err := chatService.CreateChat(r.Context(), userID, recipient.ID, "direct", nil, nil)
				if err != nil {
					log.Printf("Error creating direct chat between user %d and recipient %d: %v", userID, recipient.ID, err)
					continue
				}

				encryptedKeys := make(map[int]string)
				for _, key := range createChatReq.EncryptedKeys {
					user, err := userService.GetUserByEmail(r.Context(), key.Email)
					if err != nil {
						log.Printf("Error getting user by email %s: %v", key.Email, err)
						continue
					}
					encryptedKeys[user.ID] = key.EncryptedKey
				}

				if _, exists := encryptedKeys[userID]; !exists {
					log.Printf("Encrypted key is missing for creator user %d", userID)
					continue
				}

				if err := chatService.AddParticipants(r.Context(), chatID, []int{userID, recipient.ID}, encryptedKeys); err != nil {
					log.Printf("Error adding participants to chat %d: %v", chatID, err)
					continue
				}

				clientPool.BroadcastEvent(chatID, "new_chat", map[string]int{
					"chat_id": chatID,
				})
				log.Printf("Direct chat created with ID %d between user %d and recipient %d", chatID, userID, recipient.ID)
			case "group":
				if createChatReq.Name == nil || *createChatReq.Name == "" || len(createChatReq.Emails) == 0 || len(createChatReq.EncryptedKeys) == 0 {
					log.Printf("Invalid create_chat request from user %d: name, emails, and encrypted keys are required for group chat", userID)
					continue
				}

				userIDs, err := userService.GetUserIDsByEmails(r.Context(), createChatReq.Emails)
				if err != nil {
					log.Printf("Error getting user IDs by emails: %v", err)
					continue
				}

				userIDs = append(userIDs, userID)

				chatID, err := chatService.CreateChat(r.Context(), userID, 0, "group", createChatReq.Name, nil)
				if err != nil {
					log.Printf("Error creating group chat: %v", err)
					continue
				}

				encryptedKeys := make(map[int]string)
				for _, key := range createChatReq.EncryptedKeys {
					user, err := userService.GetUserByEmail(r.Context(), key.Email)
					if err != nil {
						log.Printf("Error getting user by email %s: %v", key.Email, err)
						continue
					}
					encryptedKeys[user.ID] = key.EncryptedKey
				}

				if err := chatService.AddParticipants(r.Context(), chatID, userIDs, encryptedKeys); err != nil {
					log.Printf("Error adding participants to chat %d: %v", chatID, err)
					continue
				}

				clientPool.BroadcastEvent(chatID, "new_chat", map[string]int{
					"chat_id": chatID,
				})
				log.Printf("Group chat created with ID %d and name %s", chatID, *createChatReq.Name)
			case "channel":
				if createChatReq.Name == nil || *createChatReq.Name == "" || len(createChatReq.Emails) == 0 || len(createChatReq.EncryptedKeys) == 0 || createChatReq.RawAESKey == nil {
					log.Printf("Invalid create_channel request from user %d: name, emails, encrypted keys, and raw_aes_key are required for channel", userID)
					continue
				}

				userIDs, err := userService.GetUserIDsByEmails(r.Context(), createChatReq.Emails)
				if err != nil {
					log.Printf("Error getting user IDs by emails: %v", err)
					continue
				}

				userIDs = append(userIDs, userID)

				chatID, err := chatService.CreateChat(r.Context(), userID, 0, "channel", createChatReq.Name, createChatReq.RawAESKey)
				if err != nil {
					log.Printf("Error creating channel: %v", err)
					continue
				}

				encryptedKeys := make(map[int]string)
				for _, key := range createChatReq.EncryptedKeys {
					user, err := userService.GetUserByEmail(r.Context(), key.Email)
					if err != nil {
						log.Printf("Error getting user by email %s: %v", key.Email, err)
						continue
					}
					encryptedKeys[user.ID] = key.EncryptedKey
				}

				if err := chatService.AddParticipants(r.Context(), chatID, userIDs, encryptedKeys); err != nil {
					log.Printf("Error adding participants to channel %d: %v", chatID, err)
					continue
				}

				clientPool.BroadcastEvent(chatID, "new_chat", map[string]int{
					"chat_id": chatID,
				})
				log.Printf("Channel created with ID %d and name %s", chatID, *createChatReq.Name)

			default:
				log.Printf("Invalid chat type: %s", createChatReq.Type)
			}

		case "message_read":
			var rawMsg struct {
				Event string          `json:"event"`
				Data  json.RawMessage `json:"data"`
			}
			var readMsgReq struct {
				ChatID            int `json:"chat_id"`
				LastReadMessageID int `json:"last_read_message_id"`
			}

			err := json.Unmarshal([]byte(msg.Content), &rawMsg)
			if err != nil {
				log.Printf("Invalid message format from user %d: %v", userID, err)
				continue
			}

			err = json.Unmarshal(rawMsg.Data, &readMsgReq)
			if err != nil || readMsgReq.ChatID == 0 || readMsgReq.LastReadMessageID == 0 {
				log.Printf("Invalid read_messages request from user %d: %v", userID, err)
				continue
			}

			log.Printf("User %d marked messages as read in chat %d up to message ID %d",
				userID, readMsgReq.ChatID, readMsgReq.LastReadMessageID)

			messageIDs, senderIDs, err := chatService.MarkMessagesAsRead(
				r.Context(), readMsgReq.ChatID, userID, readMsgReq.LastReadMessageID)
			if err != nil {
				log.Printf("Error marking messages as read in chat %d for user %d: %v", readMsgReq.ChatID, userID, err)
				continue
			}

			if len(messageIDs) == 0 {
				log.Printf("No unread messages found in chat %d for user %d", readMsgReq.ChatID, userID)
				continue
			}

			readAt := time.Now().UTC().Format(time.RFC3339)
			eventData := map[string]interface{}{
				"chat_id":              readMsgReq.ChatID,
				"message_ids":          messageIDs,
				"last_read_message_id": readMsgReq.LastReadMessageID,
				"read_at":              readAt,
			}

			for _, senderID := range senderIDs {
				if senderID == userID {
					continue
				}

				client := clientPool.GetClient(senderID)
				if client == nil {
					log.Printf("Client %d not found in pool", senderID)
					continue
				}

				err := client.Conn.WriteJSON(map[string]interface{}{
					"event": "message_read",
					"data":  eventData,
				})
				if err != nil {
					log.Printf("Error sending message_read event to user %d: %v", senderID, err)
					client.Conn.Close()
					clientPool.RemoveClient(senderID)
				} else {
					log.Printf("Sent message_read event to user %d", senderID)
				}
			}

			log.Printf("User %d marked messages [%v] as read in chat %d", userID, messageIDs, readMsgReq.ChatID)
		}
	}
}
