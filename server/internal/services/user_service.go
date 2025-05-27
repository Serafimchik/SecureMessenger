package services

import (
	"SecureMessenger/server/internal/utils"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"SecureMessenger/server/internal/db"
	"SecureMessenger/server/internal/models"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx"
)

type UserService interface {
	CheckUserExists(ctx context.Context, username, email string) (bool, error)
	CreateUser(ctx context.Context, user *models.User) (int, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserById(ctx context.Context, id int) (*models.User, error)
	UpdateUser(ctx context.Context, id int, updatedUser *models.User) error
	DeleteUser(ctx context.Context, id int) error
	SavePublicKey(ctx context.Context, userID int, publicKey string) error
	IncrementFailedLoginAttempts(ctx context.Context, userID int) (*models.User, error)
	LockAccount(ctx context.Context, userID int, duration time.Duration) error
	ResetFailedLoginAttempts(ctx context.Context, userID int) error
	UnlockAccount(ctx context.Context, userID int) error
	SearchUsers(ctx context.Context, searchTerm string) ([]models.User, error)
	GetUserPublicKey(ctx context.Context, userID int) (string, error)
	GetUserIDsByEmails(ctx context.Context, emails []string) ([]int, error)
	GetUsersByEmails(ctx context.Context, emails []string) ([]*models.User, error)
}

type userService struct{}

func NewUserService() UserService {
	return &userService{}
}

func (us *userService) CheckUserExists(ctx context.Context, username, email string) (bool, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("COUNT(*)").
		From("users").
		Where(squirrel.Or{
			squirrel.Eq{"username": username},
			squirrel.Eq{"email": email},
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
		log.Printf("Error executing query: %v", err)
		return false, err
	}

	return count > 0, nil
}

func (us *userService) CreateUser(ctx context.Context, user *models.User) (int, error) {
	hashedPassword, err := utils.HashPassword(user.PasswordHash)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		return 0, err
	}

	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Insert("users").
		Columns("username", "email", "password_hash").
		Values(user.Username, user.Email, hashedPassword).
		Suffix("RETURNING id")

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return 0, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var userId int
	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(&userId)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		return 0, err
	}

	log.Printf("User created: %s (ID: %d)", user.Username, userId)
	return userId, nil
}

func (us *userService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("id", "username", "email", "password_hash", "public_key", "failed_attempts", "locked_until").
		From("users").
		Where(squirrel.Eq{"email": email})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var user models.User
	var publicKey sql.NullString
	var lockedUntil sql.NullTime

	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&publicKey,
		&user.FailedAttempts,
		&lockedUntil,
	)
	if err != nil {
		log.Printf("Error fetching user by email: %v", err)
		return nil, errors.New("user not found")
	}

	if publicKey.Valid {
		user.PublicKey = &publicKey.String
	} else {
		user.PublicKey = nil
	}

	if lockedUntil.Valid {
		user.LockedUntil = &lockedUntil.Time
	} else {
		user.LockedUntil = nil
	}

	log.Printf("User found: %s (ID: %d)", user.Username, user.ID)
	return &user, nil
}

func (us *userService) GetUserById(ctx context.Context, id int) (*models.User, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("id", "username", "email", "password_hash", "public_key", "failed_attempts", "locked_until").
		From("users").
		Where(squirrel.Eq{"id": id})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var user models.User
	var publicKey sql.NullString
	var lockedUntil sql.NullTime

	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&publicKey,
		&user.FailedAttempts,
		&lockedUntil,
	)
	if err != nil {
		log.Printf("Error fetching user by ID: %v", err)
		return nil, errors.New("user not found")
	}

	if publicKey.Valid {
		user.PublicKey = &publicKey.String
	} else {
		user.PublicKey = nil
	}

	if lockedUntil.Valid {
		user.LockedUntil = &lockedUntil.Time
	} else {
		user.LockedUntil = nil
	}

	log.Printf("User found: %s (ID: %d)", user.Username, user.ID)
	return &user, nil
}

func (us *userService) UpdateUser(ctx context.Context, id int, updatedUser *models.User) error {
	setClause := squirrel.Eq{}
	if updatedUser.Username != "" {
		setClause["username"] = updatedUser.Username
	}
	if updatedUser.Email != "" {
		setClause["email"] = updatedUser.Email
	}
	if updatedUser.PasswordHash != "" {
		hashedPassword, err := utils.HashPassword(updatedUser.PasswordHash)
		if err != nil {
			log.Printf("Failed to hash password: %v", err)
			return err
		}
		setClause["password_hash"] = hashedPassword
	}

	if len(setClause) == 0 {
		return errors.New("nothing to update")
	}

	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Update("users").
		SetMap(setClause).
		Where(squirrel.Eq{"id": id}).
		Suffix("RETURNING id")

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var userId int
	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(&userId)
	if err != nil {
		log.Printf("Error updating user: %v", err)
		return err
	}

	log.Printf("User updated: ID %d", id)
	return nil
}

func (us *userService) DeleteUser(ctx context.Context, id int) error {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Delete("users").
		Where(squirrel.Eq{"id": id})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	result, err := db.Pool.Exec(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error deleting user: %v", err)
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	log.Printf("User deleted: ID %d", id)
	return nil
}

func (us *userService) SavePublicKey(ctx context.Context, userID int, publicKey string) error {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Update("users").
		Set("public_key", publicKey).
		Where(squirrel.Eq{"id": userID})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	result, err := db.Pool.Exec(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error saving public key for user %d: %v", userID, err)
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	log.Printf("Public key saved for user ID %d", userID)
	return nil
}

func (us *userService) IncrementFailedLoginAttempts(ctx context.Context, userID int) (*models.User, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Update("users").
		Set("failed_attempts", squirrel.Expr("failed_attempts + 1")).
		Where(squirrel.Eq{"id": userID}).
		Suffix("RETURNING id, username, email, password_hash, public_key, failed_attempts, locked_until")

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var user models.User
	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.PublicKey,
		&user.FailedAttempts,
		&user.LockedUntil,
	)
	if err != nil {
		log.Printf("Error incrementing failed login attempts for user %d: %v", userID, err)
		return nil, err
	}

	log.Printf("Failed login attempt incremented for user %d (new failed_attempts: %d)", user.ID, user.FailedAttempts)
	return &user, nil
}

func (us *userService) ResetFailedLoginAttempts(ctx context.Context, userID int) error {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Update("users").
		Set("failed_attempts", 0).
		Set("locked_until", nil).
		Where(squirrel.Eq{"id": userID})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	result, err := db.Pool.Exec(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error resetting failed login attempts for user %d: %v", userID, err)
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	log.Printf("Failed login attempts reset for user %d", userID)
	return nil
}

func (us *userService) LockAccount(ctx context.Context, userID int, duration time.Duration) error {
	durationStr := duration.String()

	query := fmt.Sprintf("UPDATE users SET locked_until = NOW() + INTERVAL '%s' WHERE id = $1", durationStr)

	log.Printf("Executing SQL: %s", query)

	result, err := db.Pool.Exec(ctx, query, userID)
	if err != nil {
		log.Printf("Error locking account for user %d: %v", userID, err)
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	log.Printf("Account locked for user %d for %v", userID, duration)
	return nil
}

func (us *userService) UnlockAccount(ctx context.Context, userID int) error {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Update("users").
		Set("locked_until", nil).
		Where(squirrel.Eq{"id": userID})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	result, err := db.Pool.Exec(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error unlocking account for user %d: %v", userID, err)
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	log.Printf("Account unlocked for user %d", userID)
	return nil
}

func (us *userService) SearchUsers(ctx context.Context, searchTerm string) ([]models.User, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("id", "username", "email").
		From("users").
		Where(squirrel.Or{
			squirrel.Like{"username": "%" + searchTerm + "%"},
			squirrel.Like{"email": "%" + searchTerm + "%"},
		}).
		OrderBy("id DESC")

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	rows, err := db.Pool.Query(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error searching users: %v", err)
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(&user.ID, &user.Username, &user.Email)
		if err != nil {
			log.Printf("Error scanning user row: %v", err)
			continue
		}
		users = append(users, user)
	}

	if len(users) == 0 {
		log.Println("No users found for search term:", searchTerm)
		return nil, errors.New("no users found")
	}

	log.Printf("Users found for search term '%s': %+v", searchTerm, users)
	return users, nil
}

func (us *userService) GetUserPublicKey(ctx context.Context, userID int) (string, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("public_key").
		From("users").
		Where(squirrel.Eq{"id": userID})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return "", err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var publicKey string
	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(&publicKey)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Printf("Public key not found for user %d", userID)
			return "", models.ErrUserNotFound
		}
		log.Printf("Error fetching public key for user %d: %v", userID, err)
		return "", err
	}

	log.Printf("Fetched public key for user %d", userID)
	return publicKey, nil
}

func (us *userService) GetUserIDsByEmails(ctx context.Context, emails []string) ([]int, error) {
	if len(emails) == 0 {
		return nil, errors.New("no emails provided")
	}

	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("id").
		From("users").
		Where(squirrel.Eq{"email": emails})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	rows, err := db.Pool.Query(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error fetching user IDs by emails: %v", err)
		return nil, err
	}
	defer rows.Close()

	var userIDs []int
	for rows.Next() {
		var userID int
		if err := rows.Scan(&userID); err != nil {
			log.Printf("Error scanning user ID: %v", err)
			continue
		}
		userIDs = append(userIDs, userID)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating over rows: %v", err)
		return nil, err
	}

	if len(userIDs) == 0 {
		log.Printf("No users found for emails: %v", emails)
		return nil, errors.New("no users found for the provided emails")
	}

	log.Printf("Fetched user IDs: %v for emails: %v", userIDs, emails)
	return userIDs, nil
}

func (us *userService) GetUsersByEmails(ctx context.Context, emails []string) ([]*models.User, error) {
	if len(emails) == 0 {
		return nil, errors.New("no emails provided")
	}

	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("id", "username", "email", "public_key").
		From("users").
		Where(squirrel.Eq{"email": emails})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	rows, err := db.Pool.Query(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error fetching users by emails: %v", err)
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.PublicKey)
		if err != nil {
			log.Printf("Error scanning user row: %v", err)
			continue
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating over rows: %v", err)
		return nil, err
	}

	log.Printf("Fetched %d users for emails: %v", len(users), emails)
	return users, nil
}
