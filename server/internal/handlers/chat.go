package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"SecureMessenger/server/internal/models"
	"SecureMessenger/server/internal/services"
)

var chatService services.ChatService

func init() {
	userService := services.NewUserService()
	chatService = services.NewChatService(userService)
}

func CreateChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Type           string   `json:"type"`
		Name           *string  `json:"name"`
		RecipientEmail *string  `json:"recipient_email"`
		Emails         []string `json:"emails"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Type == "" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	currentUserIDRaw := ctx.Value("user_id")
	if currentUserIDRaw == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	currentUserID, ok := currentUserIDRaw.(int)
	if !ok {
		http.Error(w, "Invalid user ID in context", http.StatusInternalServerError)
		return
	}

	switch req.Type {
	case "direct":
		if req.RecipientEmail == nil || *req.RecipientEmail == "" {
			http.Error(w, "Recipient email is required for direct chat", http.StatusBadRequest)
			return
		}

		recipient, err := userService.GetUserByEmail(ctx, *req.RecipientEmail)
		if err != nil {
			log.Printf("Error getting user by email %s: %v", *req.RecipientEmail, err)
			if errors.Is(err, models.ErrUserNotFound) {
				http.Error(w, "Recipient not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		existingChatID, err := chatService.CheckExistingPrivateChat(ctx, currentUserID, recipient.ID)
		if err != nil {
			log.Printf("Error checking existing private chat: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if existingChatID > 0 {
			log.Printf("Existing private chat found with ID %d", existingChatID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]int{"chat_id": existingChatID})
			return
		}

		chatID, err := chatService.CreateChat(ctx, currentUserID, recipient.ID, "direct", nil)
		if err != nil {
			log.Printf("Error creating direct chat between user %d and recipient %d: %v", currentUserID, recipient.ID, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]int{"chat_id": chatID})

	case "group":
		if req.Name == nil || *req.Name == "" || len(req.Emails) == 0 {
			http.Error(w, "Name and emails are required for group chat", http.StatusBadRequest)
			return
		}

		userIDs, err := userService.GetUserIDsByEmails(ctx, req.Emails)
		if err != nil {
			log.Printf("Error getting user IDs by emails: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		userIDs = append(userIDs, currentUserID)

		chatID, err := chatService.CreateChat(ctx, currentUserID, 0, "group", req.Name)
		if err != nil {
			log.Printf("Error creating group chat: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if err := chatService.AddParticipants(ctx, chatID, userIDs); err != nil {
			log.Printf("Error adding participants to chat %d: %v", chatID, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]int{"chat_id": chatID})

	default:
		http.Error(w, "Invalid chat type", http.StatusBadRequest)
	}
}

func GetChatsByUserId(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	currentUserIDRaw := ctx.Value("user_id")
	if currentUserIDRaw == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	currentUserID, ok := currentUserIDRaw.(int)
	if !ok {
		http.Error(w, "Invalid user ID in context", http.StatusInternalServerError)
		return
	}

	chats, err := chatService.GetChatsByUserId(ctx, currentUserID)
	if err != nil {
		log.Printf("Error getting chats for user %d: %v", currentUserID, err)
		if errors.Is(err, models.ErrChatNotFound) {
			http.Error(w, "No chats found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	for i, chat := range chats {
		participants, err := chatService.GetParticipants(ctx, chat.ID)
		if err != nil {
			log.Printf("Error getting participants for chat %d: %v", chat.ID, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		chats[i].Participants = participants
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chats)
}

func GetChatById(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/chats/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		log.Println("Missing chat ID in URL")
		http.Error(w, "Missing chat ID in URL", http.StatusBadRequest)
		return
	}

	chatIDStr := parts[0]
	log.Printf("Received chat ID: %s", chatIDStr)

	chatID, err := strconv.Atoi(chatIDStr)
	if err != nil || chatID <= 0 {
		log.Printf("Invalid chat ID: %s", chatIDStr)
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	currentUserIDRaw := ctx.Value("user_id")
	if currentUserIDRaw == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	currentUserID, ok := currentUserIDRaw.(int)
	if !ok {
		http.Error(w, "Invalid user ID in context", http.StatusInternalServerError)
		return
	}

	isParticipant, err := chatService.IsUserParticipant(ctx, chatID, currentUserID)
	if err != nil {
		log.Printf("Error checking if user %d is a participant of chat %d: %v", currentUserID, chatID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !isParticipant {
		http.Error(w, "User is not a participant of this chat", http.StatusForbidden)
		return
	}
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 20
	}

	offset := (page - 1) * limit

	messages, err := chatService.GetMessagesByChatId(ctx, chatID, offset, limit)
	if err != nil {
		log.Printf("Error getting messages for chat %d: %v", chatID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	participants, err := chatService.GetParticipants(ctx, chatID)
	if err != nil {
		log.Printf("Error getting participants for chat %d: %v", chatID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	filteredParticipants := make([]models.User, 0)
	for _, participant := range participants {
		if participant.ID != currentUserID {
			filteredParticipants = append(filteredParticipants, participant)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"chat_id":      chatID,
		"messages":     messages,
		"participants": filteredParticipants,
	})
}

func AddParticipant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/chats/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		log.Println("Missing chat ID in URL")
		http.Error(w, "Missing chat ID in URL", http.StatusBadRequest)
		return
	}

	chatIDStr := parts[0]
	chatID, err := strconv.Atoi(chatIDStr)
	if err != nil || chatID <= 0 {
		log.Printf("Invalid chat ID: %s", chatIDStr)
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := userService.GetUserByEmail(ctx, req.Email)
	if err != nil {
		log.Printf("Error getting user by email: %v", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	err = chatService.AddParticipant(ctx, chatID, user.ID)
	if err != nil {
		log.Printf("Error adding participant to chat %d: %v", chatID, err)
		http.Error(w, "Failed to add participant", http.StatusInternalServerError)
		return
	}

	participants, err := chatService.GetParticipants(ctx, chatID)
	if err != nil {
		log.Printf("Error getting participants for chat %d: %v", chatID, err)
	} else {
		eventData := map[string]interface{}{
			"action":       "add_participant",
			"chat_id":      chatID,
			"user_id":      user.ID,
			"username":     user.Username,
			"participants": participants,
		}
		broadcastToChat(chatID, "participant_added", eventData)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Participant added successfully",
	})
}

func RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/chats/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		log.Println("Missing chat ID in URL")
		http.Error(w, "Missing chat ID in URL", http.StatusBadRequest)
		return
	}

	chatIDStr := parts[0]
	chatID, err := strconv.Atoi(chatIDStr)
	if err != nil || chatID <= 0 {
		log.Printf("Invalid chat ID: %s", chatIDStr)
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	var req struct {
		UserID int `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := userService.GetUserById(ctx, req.UserID)
	if err != nil {
		log.Printf("Error getting user by ID %d: %v", req.UserID, err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	err = chatService.RemoveParticipant(ctx, chatID, req.UserID)
	if err != nil {
		log.Printf("Error removing participant %d from chat %d: %v", req.UserID, chatID, err)
		http.Error(w, "Failed to remove participant", http.StatusInternalServerError)
		return
	}

	participants, err := chatService.GetParticipants(ctx, chatID)
	if err != nil {
		log.Printf("Error getting participants for chat %d: %v", chatID, err)
	} else {
		eventData := map[string]interface{}{
			"action":       "remove_participant",
			"chat_id":      chatID,
			"user_id":      user.ID,
			"username":     user.Username,
			"participants": participants,
		}
		broadcastToChat(chatID, "participant_removed", eventData)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Participant removed successfully",
	})
}
