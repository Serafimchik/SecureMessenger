package models

import (
	"time"
)

type Chat struct {
	ID               int       `json:"id" db:"id"`
	Type             string    `json:"type" db:"type"`
	Name             string    `json:"name,omitempty" db:"name"`
	CreatedBy        int       `json:"created_by" db:"created_by"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	EncryptedChatKey string    `json:"encrypted_chat_key"`
	Participants     []User    `json:"participants"`
}

type ChatParticipant struct {
	ID       int       `json:"id" db:"id"`
	ChatID   int       `json:"chat_id" db:"chat_id"`
	UserID   int       `json:"user_id" db:"user_id"`
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`
}

type ChatWithLastMessage struct {
	Chat
	LastMessageContent *string    `db:"last_message_content"`
	LastMessageSentAt  *time.Time `db:"last_message_sent_at"`
	UnreadCount        int        `json:"unread_count"`
}
