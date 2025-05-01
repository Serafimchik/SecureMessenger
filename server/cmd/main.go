package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"SecureMessenger/server/internal/appMiddleware"
	"SecureMessenger/server/internal/db"
	"SecureMessenger/server/internal/handlers"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
)

func main() {
	db.InitDB()

	r := chi.NewRouter()

	r.Use(appMiddleware.CorsMiddleware)

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/register", handlers.Register)
	r.Post("/login", handlers.Login)

	r.Group(func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware)
		r.Get("/api/profile", handlers.GetProfile)

		r.Post("/api/chats/create", handlers.CreateChat)
		r.Get("/api/chats", handlers.GetChatsByUserId)
		r.Get("/api/chats/{chat_id}", handlers.GetChatById)
		r.Post("/api/chats/{chat_id}/participants", handlers.AddParticipants)
	})

	r.Get("/ws", handlers.WebSocketHandler)

	port := ":8080"
	srv := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Server started on port %s\n", port)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %s\n", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	log.Println("Stopping the server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %s\n", err)
	}
	log.Println("Server has been successfully stopped")
}
