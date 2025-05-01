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

	"github.com/go-chi/chi"
)

var chatService services.ChatService

func init() {
	userService := services.NewUserService()
	chatService = services.NewChatService(userService)
}

func CreateChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		RecipientEmail string  `json:"recipient_email"`
		Type           string  `json:"type"`
		Name           *string `json:"name"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.RecipientEmail == "" || req.Type == "" {
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

	recipient, err := userService.GetUserByEmail(ctx, req.RecipientEmail)
	if err != nil {
		log.Printf("Error getting user by email %s: %v", req.RecipientEmail, err)
		if errors.Is(err, models.ErrUserNotFound) {
			http.Error(w, "Recipient not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	chatID, err := chatService.CreateChat(ctx, currentUserID, recipient.ID, req.Type, req.Name)
	if err != nil {
		log.Printf("Error creating chat between user %d and recipient %d: %v", currentUserID, recipient.ID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int{"chat_id": chatID})
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chats)
}

func GetChatById(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	log.Println("Handling request for GetChatById")

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
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"chat_id":  chatID,
		"messages": messages,
	})
}

func AddParticipants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chatIDStr := chi.URLParam(r, "chat_id")
	chatID, err := strconv.Atoi(chatIDStr)
	if err != nil || chatID == 0 {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
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

	isCreator, err := chatService.IsChatCreator(ctx, chatID, currentUserID)
	if err != nil {
		log.Printf("Error checking creator of chat %d: %v", chatID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !isCreator {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		UserIDs []int `json:"user_ids"`
	}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil || len(req.UserIDs) == 0 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	err = chatService.AddParticipants(ctx, chatID, req.UserIDs)
	if err != nil {
		log.Printf("Error adding participants to chat %d: %v", chatID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Participants added"})
}
