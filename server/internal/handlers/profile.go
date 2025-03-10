package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

func GetProfile(w http.ResponseWriter, r *http.Request) {
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

	user, err := userService.GetUserById(userID)
	if err != nil {
		log.Printf("Error fetching user profile: %v", err)
		if err.Error() == "user not found" {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
