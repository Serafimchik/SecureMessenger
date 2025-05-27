package services

import (
	"context"
	"errors"
	"log"
	"time"

	"SecureMessenger/server/internal/db"
	"SecureMessenger/server/internal/models"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx"
)

type ChatService interface {
	CreateChat(ctx context.Context, creatorID, recipientID int, chatType string, chatName *string) (int, error)
	AddParticipants(ctx context.Context, chatID int, userIDs []int, encryptedKeys map[int]string) error
	GetChatsByUserId(ctx context.Context, userID int) ([]models.ChatWithLastMessage, error)
	GetChatById(ctx context.Context, chatID, userID int) (*models.Chat, error)
	IsUserInChat(ctx context.Context, chatID, userID int) (bool, error)
	IsChatCreator(ctx context.Context, chatID, userID int) (bool, error)
	GetParticipantsByChatId(ctx context.Context, chatID int) ([]models.User, error)
	SaveMessage(ctx context.Context, chatID, senderID int, username, content string) (int, time.Time, error)
	GetMessagesByChatId(ctx context.Context, chatID, offset, limit int) ([]models.Message, error)
	IsUserParticipant(ctx context.Context, chatID, userID int) (bool, error)
	MarkMessagesAsRead(ctx context.Context, chatID, recipientID, lastReadMessageID int) ([]int, []int, error)
	GetUnreadMessagesCount(ctx context.Context, chatID, userID int) (int, error)
	GetParticipants(ctx context.Context, chatID int) ([]models.User, error)
	CheckExistingPrivateChat(ctx context.Context, user1ID, user2ID int) (int, error)
	RemoveParticipant(ctx context.Context, chatID, userID int) error
	AddParticipant(ctx context.Context, chatID, userID int) error
}

type chatService struct {
	UserService UserService
}

func NewChatService(userService UserService) *chatService {
	return &chatService{
		UserService: userService,
	}
}

func (cs *chatService) CreateChat(ctx context.Context, creatorID, recipientID int, chatType string, chatName *string) (int, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Insert("chats").
		Columns("type", "name", "created_by", "created_at").
		Values(chatType, chatName, creatorID, time.Now()).
		Suffix("RETURNING id")
	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return 0, err
	}
	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)
	var chatID int
	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(&chatID)
	if err != nil {
		log.Printf("Error creating chat: %v", err)
		return 0, err
	}
	log.Printf("Chat created with ID %d", chatID)

	if chatType == "direct" {
		encryptedKeys := make(map[int]string)
		err = cs.AddParticipants(ctx, chatID, []int{creatorID, recipientID}, encryptedKeys)
		if err != nil {
			log.Printf("Error adding participants to chat %d: %v", chatID, err)
			return 0, err
		}
	}
	return chatID, nil
}

func (cs *chatService) AddParticipants(ctx context.Context, chatID int, userIDs []int, encryptedKeys map[int]string) error {
	for _, userID := range userIDs {
		encryptedKey := encryptedKeys[userID]
		if encryptedKey == "" {
			log.Printf("Encrypted key is missing for user %d in chat %d", userID, chatID)
			continue
		}

		isParticipant, err := cs.IsUserInChat(ctx, chatID, userID)
		if err != nil {
			log.Printf("Error checking if user %d is in chat %d: %v", userID, chatID, err)
			return err
		}
		if isParticipant {
			log.Printf("User %d is already a participant of chat %d", userID, chatID)
			continue
		}

		query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
			Insert("chat_participants").
			Columns("chat_id", "user_id", "encrypted_chat_key").
			Values(chatID, userID, encryptedKey)
		sqlStr, args, err := query.ToSql()
		if err != nil {
			log.Printf("Failed to build SQL query: %v", err)
			return err
		}
		log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)
		_, err = db.Pool.Exec(ctx, sqlStr, args...)
		if err != nil {
			log.Printf("Error adding participant %d to chat %d: %v", userID, chatID, err)
			return err
		}
	}
	log.Printf("Participants added to chat %d", chatID)
	return nil
}

func (cs *chatService) GetChatsByUserId(ctx context.Context, userID int) ([]models.ChatWithLastMessage, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("chats.id", "chats.type",
			"COALESCE(chats.name, '') AS name",
			"chats.created_by", "chats.created_at",
			"COALESCE(messages.content, '') AS last_message_content",
			"COALESCE(messages.sent_at, '1970-01-01T00:00:01Z'::timestamp) AS last_message_sent_at").
		From("chats").
		Join("chat_participants ON chats.id = chat_participants.chat_id").
		LeftJoin("messages ON chats.id = messages.chat_id AND messages.sent_at = (" +
			"SELECT MAX(sent_at) FROM messages WHERE messages.chat_id = chats.id)").
		Where(squirrel.Eq{"chat_participants.user_id": userID}).
		OrderBy("messages.sent_at DESC NULLS LAST")

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	rows, err := db.Pool.Query(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error getting chats for user %d: %v", userID, err)
		return nil, err
	}
	defer rows.Close()

	var chats []models.ChatWithLastMessage
	for rows.Next() {
		var chat models.ChatWithLastMessage
		err := rows.Scan(&chat.ID, &chat.Type, &chat.Name, &chat.CreatedBy, &chat.CreatedAt,
			&chat.LastMessageContent, &chat.LastMessageSentAt)
		if err != nil {
			log.Printf("Error scanning chat row: %v", err)
			continue
		}
		chats = append(chats, chat)
	}

	if len(chats) == 0 {
		log.Printf("No chats found for user %d", userID)
		return nil, models.ErrChatNotFound
	}

	for i, chat := range chats {
		if chat.Type == "direct" {
			participants, err := cs.GetParticipantsByChatId(ctx, chat.ID)
			if err != nil {
				log.Printf("Error getting participants for chat %d: %v", chat.ID, err)
				continue
			}

			var recipientID int
			for _, participant := range participants {
				if participant.ID != userID {
					recipientID = participant.ID
					break
				}
			}

			if recipientID != 0 {
				recipient, err := cs.UserService.GetUserById(ctx, recipientID)
				if err != nil {
					log.Printf("Error getting recipient by ID %d: %v", recipientID, err)
					continue
				}
				chats[i].Name = recipient.Username
			}
		}

		unreadCount, err := cs.GetUnreadMessagesCount(ctx, chat.ID, userID)
		if err != nil {
			log.Printf("Error getting unread messages count for chat %d: %v", chat.ID, err)
			continue
		}
		chats[i].UnreadCount = unreadCount
	}

	log.Printf("Chats retrieved for user %d: %+v", userID, chats)
	return chats, nil
}

func (cs *chatService) GetChatById(ctx context.Context, chatID, userID int) (*models.Chat, error) {
	isParticipant, err := cs.IsUserInChat(ctx, chatID, userID)
	if err != nil {
		log.Printf("Error checking user %d in chat %d: %v", userID, chatID, err)
		return nil, err
	}

	if !isParticipant {
		log.Printf("User %d is not a participant of chat %d", userID, chatID)
		return nil, errors.New("user not a participant")
	}

	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("id", "type", "name", "created_by", "created_at").
		From("chats").
		Where(squirrel.Eq{"id": chatID})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var chat models.Chat
	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(&chat.ID, &chat.Type, &chat.Name, &chat.CreatedBy, &chat.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Printf("Chat %d not found", chatID)
			return nil, errors.New("chat not found")
		}
		log.Printf("Error getting chat %d: %v", chatID, err)
		return nil, err
	}

	log.Printf("Chat retrieved: %+v", chat)
	return &chat, nil
}

func (cs *chatService) IsUserInChat(ctx context.Context, chatID, userID int) (bool, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("COUNT(*)").
		From("chat_participants").
		Where(squirrel.And{
			squirrel.Eq{"chat_id": chatID},
			squirrel.Eq{"user_id": userID},
		})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return false, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var count int
	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(&count)
	if err != nil {
		log.Printf("Error checking user %d in chat %d: %v", userID, chatID, err)
		return false, err
	}

	return count > 0, nil
}

func (cs *chatService) IsChatCreator(ctx context.Context, chatID, userID int) (bool, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("created_by").
		From("chats").
		Where(squirrel.Eq{"id": chatID})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return false, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var createdBy int
	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(&createdBy)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Printf("Chat %d not found", chatID)
			return false, models.ErrChatNotFound
		}
		log.Printf("Error getting creator of chat %d: %v", chatID, err)
		return false, err
	}

	return createdBy == userID, nil
}

func (cs *chatService) GetParticipantsByChatId(ctx context.Context, chatID int) ([]models.User, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("users.id", "users.username", "users.email", "users.public_key").
		From("users").
		Join("chat_participants ON users.id = chat_participants.user_id").
		Where(squirrel.Eq{"chat_participants.chat_id": chatID})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	rows, err := db.Pool.Query(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error getting participants for chat %d: %v", chatID, err)
		return nil, err
	}
	defer rows.Close()

	var participants []models.User
	for rows.Next() {
		var participant models.User
		err := rows.Scan(&participant.ID, &participant.Username, &participant.Email, &participant.PublicKey)
		if err != nil {
			log.Printf("Error scanning participant row: %v", err)
			continue
		}
		participants = append(participants, participant)
	}

	if len(participants) == 0 {
		log.Printf("No participants found for chat %d", chatID)
		return nil, errors.New("no participants found")
	}

	log.Printf("Participants retrieved for chat %d: %+v", chatID, participants)
	return participants, nil
}

func (cs *chatService) SaveMessage(ctx context.Context, chatID, senderID int, username, content string) (int, time.Time, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Insert("messages").
		Columns("chat_id", "sender_id", "username", "content", "encrypted", "sent_at").
		Values(chatID, senderID, username, content, true, squirrel.Expr("NOW()")).
		Suffix("RETURNING id, sent_at")

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return 0, time.Time{}, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var messageID int
	var sentAt time.Time
	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(&messageID, &sentAt)
	if err != nil {
		log.Printf("Error saving message: %v", err)
		return 0, time.Time{}, err
	}

	log.Printf("Message saved: Chat ID %d, Sender ID %d (%s), Message ID: %d, Sent At: %v", chatID, senderID, username, messageID, sentAt)
	return messageID, sentAt, nil
}

func (cs *chatService) GetMessagesByChatId(ctx context.Context, chatID, offset, limit int) ([]models.Message, error) {
	queryBuilder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("id", "chat_id", "sender_id", "username", "content", "sent_at", "read_at").
		From("messages").
		Where(squirrel.Eq{"chat_id": chatID}).
		OrderBy("sent_at DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset))

	sqlQuery, args, err := queryBuilder.ToSql()
	if err != nil {
		log.Printf("Error building SQL query: %v", err)
		return nil, errors.New("failed to build query")
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlQuery, args)

	rows, err := db.Pool.Query(ctx, sqlQuery, args...)
	if err != nil {
		log.Printf("Error executing query for chat %d: %v", chatID, err)
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message

	for rows.Next() {
		var msg models.Message
		var readAt pgtype.Timestamptz

		err := rows.Scan(&msg.ID, &msg.ChatID, &msg.SenderID, &msg.Username, &msg.Content, &msg.SentAt, &readAt)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			return nil, err
		}

		if readAt.Status == pgtype.Present {
			msg.ReadAt = &readAt.Time
		} else {
			msg.ReadAt = nil
		}

		messages = append(messages, msg)
	}

	if rows.Err() != nil {
		log.Printf("Error after iterating rows: %v", rows.Err())
		return nil, rows.Err()
	}

	log.Printf("Fetched %d messages for chat %d", len(messages), chatID)
	return messages, nil
}

func (cs *chatService) IsUserParticipant(ctx context.Context, chatID, userID int) (bool, error) {
	query := `
        SELECT EXISTS (
            SELECT 1
            FROM chat_participants
            WHERE chat_id = $1 AND user_id = $2
        )
    `

	var exists bool
	err := db.Pool.QueryRow(ctx, query, chatID, userID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking if user %d is a participant of chat %d: %v", userID, chatID, err)
		return false, err
	}

	return exists, nil
}

func (cs *chatService) MarkMessagesAsRead(ctx context.Context, chatID, recipientID, lastReadMessageID int) ([]int, []int, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("id", "sender_id").
		From("messages").
		Where(squirrel.And{
			squirrel.Eq{"chat_id": chatID},
			squirrel.LtOrEq{"id": lastReadMessageID},
			squirrel.Eq{"read_at": nil},
			squirrel.NotEq{"sender_id": recipientID},
		})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	rows, err := db.Pool.Query(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error fetching unread messages for chat %d and recipient %d: %v", chatID, recipientID, err)
		return nil, nil, err
	}
	defer rows.Close()

	var messageIDs []int
	var senderIDsMap = make(map[int]struct{})
	for rows.Next() {
		var id, senderID int
		err := rows.Scan(&id, &senderID)
		if err != nil {
			log.Printf("Error scanning message ID or sender ID: %v", err)
			continue
		}
		messageIDs = append(messageIDs, id)
		senderIDsMap[senderID] = struct{}{}
	}

	if len(messageIDs) == 0 {
		log.Printf("No unread messages found for chat %d and recipient %d", chatID, recipientID)
		return nil, nil, nil
	}

	var senderIDs []int
	for senderID := range senderIDsMap {
		senderIDs = append(senderIDs, senderID)
	}

	updateQuery := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Update("messages").
		Set("read_at", squirrel.Expr("NOW()")).
		Where(squirrel.And{
			squirrel.Eq{"chat_id": chatID},
			squirrel.LtOrEq{"id": lastReadMessageID},
			squirrel.Eq{"read_at": nil},
			squirrel.NotEq{"sender_id": recipientID},
		})

	updateSQL, updateArgs, err := updateQuery.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", updateSQL, updateArgs)

	_, err = db.Pool.Exec(ctx, updateSQL, updateArgs...)
	if err != nil {
		log.Printf("Error marking messages as read for chat %d and recipient %d: %v", chatID, recipientID, err)
		return nil, nil, err
	}

	log.Printf("Marked messages [%v] as read in chat %d for user %d", messageIDs, chatID, recipientID)
	return messageIDs, senderIDs, nil
}

func (cs *chatService) GetUnreadMessagesCount(ctx context.Context, chatID, userID int) (int, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("COUNT(*)").
		From("messages").
		Where(squirrel.And{
			squirrel.Eq{"chat_id": chatID},
			squirrel.NotEq{"sender_id": userID},
			squirrel.Eq{"read_at": nil},
		})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return 0, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var count int
	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(&count)
	if err != nil {
		log.Printf("Error getting unread messages count for chat %d and user %d: %v", chatID, userID, err)
		return 0, err
	}

	log.Printf("Unread messages count for chat %d and user %d: %d", chatID, userID, count)
	return count, nil
}

func (cs *chatService) GetParticipants(ctx context.Context, chatID int) ([]models.User, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("u.id", "u.username", "u.public_key").
		From("users u").
		Join("chat_participants cp ON u.id = cp.user_id").
		Where(squirrel.Eq{"cp.chat_id": chatID})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	rows, err := db.Pool.Query(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error fetching participants for chat %d: %v", chatID, err)
		return nil, err
	}
	defer rows.Close()

	var participants []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(&user.ID, &user.Username, &user.PublicKey)
		if err != nil {
			log.Printf("Error scanning participant: %v", err)
			continue
		}
		participants = append(participants, user)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating over participants: %v", err)
		return nil, err
	}

	log.Printf("Fetched %d participants for chat %d", len(participants), chatID)
	return participants, nil
}

func (cs *chatService) CheckExistingPrivateChat(ctx context.Context, user1ID, user2ID int) (int, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("c.id").
		From("chats c").
		Join("chat_participants cp1 ON c.id = cp1.chat_id").
		Join("chat_participants cp2 ON c.id = cp2.chat_id").
		Where(squirrel.Eq{
			"c.type":      "direct",
			"cp1.user_id": user1ID,
			"cp2.user_id": user2ID,
		})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return 0, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var chatID int
	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(&chatID)
	if err != nil {
		if err == pgx.ErrNoRows || err.Error() == "no rows in result set" {
			log.Printf("No existing private chat found between users %d and %d", user1ID, user2ID)
			return 0, nil
		}
		log.Printf("Error checking existing private chat: %v", err)
		return 0, err
	}

	log.Printf("Found existing private chat with ID %d", chatID)
	return chatID, nil
}

func (cs *chatService) RemoveParticipant(ctx context.Context, chatID, userID int) error {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Delete("chat_participants").
		Where(squirrel.And{
			squirrel.Eq{"chat_id": chatID},
			squirrel.Eq{"user_id": userID},
		})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)
	_, err = db.Pool.Exec(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error removing participant %d from chat %d: %v", userID, chatID, err)
		return err
	}

	log.Printf("Participant %d removed from chat %d", userID, chatID)
	return nil
}

func (cs *chatService) AddParticipant(ctx context.Context, chatID, userID int) error {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Insert("chat_participants").
		Columns("chat_id", "user_id").
		Values(chatID, userID)
	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return err
	}
	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)
	_, err = db.Pool.Exec(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error adding participant %d to chat %d: %v", userID, chatID, err)
		return err
	}
	log.Printf("Participant %d added to chat %d", userID, chatID)
	return nil
}
