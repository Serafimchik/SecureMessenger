package db

import (
	"context"
	"log"

	"github.com/jackc/pgx/v4/pgxpool"
)

var Pool *pgxpool.Pool

func InitDB() {
	ctx := context.Background()

	dbDSN := "host=host.docker.internal  port=5432 dbname=note user=note-user password=note-password"

	pool, err := pgxpool.Connect(ctx, dbDSN)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	Pool = pool

	log.Println("Database connection pool initialized")
}
