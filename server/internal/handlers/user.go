package handlers

import (
	"SecureMessenger/server/internal/models"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func SearchUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()
	searchTerm := query.Get("q")
	if searchTerm == "" {
		http.Error(w, "Search term is required", http.StatusBadRequest)
		return
	}

	users, err := userService.SearchUsers(ctx, searchTerm)
	if err != nil {
		log.Printf("Error searching users: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func GetPublicKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	log.Println("Handling request for GetPublicKey")

	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/users/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		log.Println("Missing user ID in URL")
		http.Error(w, "Missing user ID in URL", http.StatusBadRequest)
		return
	}

	userIDStr := parts[0]
	log.Printf("Received user ID: %s", userIDStr)

	userID, err := strconv.Atoi(userIDStr)
	if err != nil || userID <= 0 {
		log.Printf("Invalid user ID: %s", userIDStr)
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	publicKey, err := userService.GetUserPublicKey(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			log.Printf("User %d not found", userID)
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		log.Printf("Error getting public key for user %d: %v", userID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"user_id":    strconv.Itoa(userID),
		"public_key": publicKey,
	})
}
