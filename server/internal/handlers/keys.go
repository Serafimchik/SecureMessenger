package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

func SavePublicKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PublicKey string `json:"public_key"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.PublicKey == "" {
		log.Println("Invalid request body")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	userIDRaw := r.Context().Value("user_id")
	if userIDRaw == nil {
		log.Println("Missing user_id in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID, ok := userIDRaw.(int)
	if !ok {
		log.Println("Invalid user_id type in context")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	err = userService.SavePublicKey(ctx, userID, req.PublicKey)
	if err != nil {
		log.Printf("Error saving public key for user %d: %v", userID, err)
		http.Error(w, "Failed to save public key", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Public key saved"})
}
