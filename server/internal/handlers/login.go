package handlers

import (
	"SecureMessenger/server/internal/utils"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

var jwtSecret = []byte("secret-key")

func Login(w http.ResponseWriter, r *http.Request) {
	var loginData struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := json.NewDecoder(r.Body).Decode(&loginData)
	if err != nil {
		http.Error(w, "Incorrect data", http.StatusBadRequest)
		return
	}

	user, err := userService.GetUserByEmail(loginData.Email)
	if err != nil {
		http.Error(w, "User was not found", http.StatusUnauthorized)
		return
	}

	log.Printf("Checking password: %s, hash: %s", loginData.Password, user.PasswordHash)

	if err := utils.CheckPasswordHash(loginData.Password, user.PasswordHash); err != nil {
		log.Printf("Password verification failed: %v", err)
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "Token creation error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}
