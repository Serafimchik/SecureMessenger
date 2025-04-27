package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"SecureMessenger/server/internal/models"
	"SecureMessenger/server/internal/services"
)

var userService services.UserService

func init() {
	userService = services.NewUserService()
}

func Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Username  string `json:"username"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		PublicKey string `json:"public_key"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Username == "" || req.Email == "" || req.Password == "" || req.PublicKey == "" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	exists, err := userService.CheckUserExists(ctx, req.Username, req.Email)
	if err != nil {
		log.Printf("Error checking user existence: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Internal server error",
		})
		return
	}

	if exists {
		log.Println("User already exists")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "User with this email or username already exists",
		})
		return
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: req.Password,
	}

	userId, err := userService.CreateUser(ctx, user)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Failed to create user",
		})
		return
	}

	err = userService.SavePublicKey(ctx, userId, req.PublicKey)
	if err != nil {
		log.Printf("Error saving public key for user %d: %v", userId, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Failed to save public key",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User created",
		"user_id": strconv.Itoa(userId),
	})
}
