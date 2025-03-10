package services

import (
	"SecureMessenger/server/internal/utils"
	"context"
	"errors"
	"log"

	"SecureMessenger/server/internal/db"
	"SecureMessenger/server/internal/models"

	"github.com/Masterminds/squirrel"
)

type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

func (us *UserService) CheckUserExists(username, email string) (bool, error) {
	ctx := context.Background()

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

func (us *UserService) CreateUser(user *models.User) (int, error) {
	ctx := context.Background()

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

func (us *UserService) GetUserByEmail(email string) (*models.User, error) {
	ctx := context.Background()

	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("id", "username", "email", "password_hash").
		From("users").
		Where(squirrel.Eq{"email": email})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var user models.User
	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash)
	if err != nil {
		log.Printf("Error fetching user: %v", err)
		return nil, err
	}

	log.Printf("User found: %s (ID: %d)", user.Username, user.ID)
	return &user, nil
}

func (us *UserService) GetUserById(id int) (*models.User, error) {
	ctx := context.Background()

	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("id", "username", "email", "password_hash").
		From("users").
		Where(squirrel.Eq{"id": id})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return nil, err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	var user models.User
	err = db.Pool.QueryRow(ctx, sqlStr, args...).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash)
	if err != nil {
		log.Printf("Error fetching user: %v", err)
		return nil, err
	}

	log.Printf("User found: %s (ID: %d)", user.Username, user.ID)
	return &user, nil
}

func (us *UserService) UpdateUser(id int, updatedUser *models.User) error {
	ctx := context.Background()

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
		Where(squirrel.Eq{"id": id})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		log.Printf("Failed to build SQL query: %v", err)
		return err
	}

	log.Printf("Executing SQL: %s, Args: %v", sqlStr, args)

	result, err := db.Pool.Exec(ctx, sqlStr, args...)
	if err != nil {
		log.Printf("Error updating user: %v", err)
		return err
	}

	rowsAffected := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		return err
	}

	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	log.Printf("User updated: ID %d", id)
	return nil
}

func (us *UserService) DeleteUser(id int) error {
	ctx := context.Background()

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
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		return err
	}

	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	log.Printf("User deleted: ID %d", id)
	return nil
}
