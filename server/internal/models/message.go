package models

import (
	"time"
)

type Message struct {
	ID        int        `json:"id" db:"id"`
	ChatID    int        `json:"chat_id" db:"chat_id"`
	SenderID  int        `json:"sender_id" db:"sender_id"`
	Username  string     `json:"username"`
	Content   string     `json:"content" db:"content"`
	Encrypted bool       `json:"encrypted" db:"encrypted"`
	SentAt    time.Time  `json:"sent_at" db:"sent_at"`
	ReadAt    *time.Time `json:"read_at,omitempty" db:"read_at"`
}

type File struct {
	ID         int       `json:"id" db:"id"`
	MessageID  int       `json:"message_id" db:"message_id"`
	FileURL    string    `json:"file_url" db:"file_url"`
	FileName   string    `json:"file_name" db:"file_name"`
	Size       int64     `json:"size" db:"size"`
	UploadedAt time.Time `json:"uploaded_at" db:"uploaded_at"`
}

type Notification struct {
	ID        int       `json:"id" db:"id"`
	UserID    int       `json:"user_id" db:"user_id"`
	ChatID    int       `json:"chat_id" db:"chat_id"`
	Message   string    `json:"message" db:"message"`
	IsRead    bool      `json:"is_read" db:"is_read"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
