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

func GetPublicKeys(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Emails []string `json:"emails"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Emails) == 0 {
		http.Error(w, "No emails provided", http.StatusBadRequest)
		return
	}

	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		log.Printf("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	requester, err := userService.GetUserById(r.Context(), userID)
	if err != nil {
		log.Printf("Error getting user by ID %d: %v", userID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	users, err := userService.GetUsersByEmails(r.Context(), req.Emails)
	if err != nil {
		log.Printf("Error getting users by emails: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := struct {
		Keys []struct {
			Email     string `json:"email"`
			PublicKey string `json:"public_key"`
		} `json:"keys"`
	}{}

	response.Keys = append(response.Keys, struct {
		Email     string `json:"email"`
		PublicKey string `json:"public_key"`
	}{
		Email:     requester.Email,
		PublicKey: *requester.PublicKey,
	})

	for _, user := range users {
		response.Keys = append(response.Keys, struct {
			Email     string `json:"email"`
			PublicKey string `json:"public_key"`
		}{
			Email:     user.Email,
			PublicKey: *user.PublicKey,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
