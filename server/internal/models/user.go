package models

import (
	"time"
)

type User struct {
	ID             int        `json:"id" db:"id"`
	Username       string     `json:"username" db:"username"`
	Email          string     `json:"email" db:"email"`
	PasswordHash   string     `json:"-" db:"password_hash"`
	PublicKey      *string    `json:"public_key,omitempty" db:"public_key"`
	FailedAttempts int        `json:"-" db:"failed_attempts"`
	LockedUntil    *time.Time `json:"-" db:"locked_until"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
}

type RefreshToken struct {
	ID        int       `json:"id" db:"id"`
	UserID    int       `json:"user_id" db:"user_id"`
	Token     string    `json:"token" db:"token"`
	Expiry    int64     `json:"expiry" db:"expiry"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type EncryptedKey struct {
	Email        string `json:"email"`
	EncryptedKey string `json:"encrypted_key"`
}
