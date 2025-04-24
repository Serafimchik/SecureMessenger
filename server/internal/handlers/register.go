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
		Username     string `json:"username"`
		Email        string `json:"email"`
		PasswordHash string `json:"password"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Username == "" || req.Email == "" || req.PasswordHash == "" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	exists, err := userService.CheckUserExists(ctx, req.Username, req.Email)
	if err != nil {
		log.Printf("Error checking user existence: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if exists {
		log.Println("User already exists")
		http.Error(w, "User with this email or username already exists", http.StatusConflict)
		return
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: req.PasswordHash,
	}

	userId, err := userService.CreateUser(ctx, user)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "User created", "user_id": strconv.Itoa(userId)})
}
