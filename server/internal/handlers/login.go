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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "User not found",
			})
			return
		}
		log.Printf("Error fetching user by email: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Internal server error",
		})
		return
	}

	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		log.Printf("Account is locked until %v for user %d", user.LockedUntil, user.ID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Account is temporarily locked due to multiple failed login attempts",
		})
		return
	}

	if err := utils.CheckPasswordHash(loginData.Password, user.PasswordHash); err != nil {
		log.Printf("Password verification failed for user %d", user.ID)

		updatedUser, err := userService.IncrementFailedLoginAttempts(ctx, user.ID)
		if err != nil {
			log.Printf("Error incrementing failed login attempts for user %d: %v", user.ID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Internal server error",
			})
			return
		}

		if updatedUser.FailedAttempts >= 5 {
			err = userService.LockAccount(ctx, updatedUser.ID, 5*time.Minute)
			if err != nil {
				log.Printf("Error locking account for user %d: %v", updatedUser.ID, err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"message": "Internal server error",
				})
				return
			}

			log.Printf("Account locked for user %d for 5 minutes", updatedUser.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Account is temporarily locked due to multiple failed login attempts",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Invalid email or password",
		})
		return
	}

	err = userService.ResetFailedLoginAttempts(ctx, user.ID)
	if err != nil {
		log.Printf("Error resetting failed login attempts for user %d: %v", user.ID, err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Printf("Error creating token for user %d: %v", user.ID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Token creation error",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}
