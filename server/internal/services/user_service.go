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
