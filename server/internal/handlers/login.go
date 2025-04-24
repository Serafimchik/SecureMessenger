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
	if err != nil || loginData.Email == "" || loginData.Password == "" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	user, err := userService.GetUserByEmail(ctx, loginData.Email)
	if err != nil {
		if err.Error() == "user not found" {
			log.Printf("User with email %s not found", loginData.Email)
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}
		log.Printf("Error fetching user by email: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		log.Printf("Account is locked until %v for user %d", user.LockedUntil, user.ID)
		http.Error(w, "Account is temporarily locked due to multiple failed login attempts", http.StatusUnauthorized)
		return
	}

	if err := utils.CheckPasswordHash(loginData.Password, user.PasswordHash); err != nil {
		log.Printf("Password verification failed for user %d", user.ID)

		updatedUser, err := userService.IncrementFailedLoginAttempts(ctx, user.ID)
		if err != nil {
			log.Printf("Error incrementing failed login attempts for user %d: %v", user.ID, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if updatedUser.FailedAttempts >= 5 {
			err = userService.LockAccount(ctx, updatedUser.ID, 5*time.Minute)
			if err != nil {
				log.Printf("Error locking account for user %d: %v", updatedUser.ID, err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			log.Printf("Account locked for user %d for 5 minutes", updatedUser.ID)
			http.Error(w, "Account is temporarily locked due to multiple failed login attempts", http.StatusUnauthorized)
			return
		}

		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	err = userService.ResetFailedLoginAttempts(ctx, user.ID)
	if err != nil {
		log.Printf("Error resetting failed login attempts for user %d: %v", user.ID, err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Printf("Error creating token for user %d: %v", user.ID, err)
		http.Error(w, "Token creation error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}
