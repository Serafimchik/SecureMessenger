package models

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrChatNotFound       = errors.New("chat not found")
	ErrUserNotParticipant = errors.New("user is not a participant")
)
