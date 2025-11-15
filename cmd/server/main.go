package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	httpapi "github.com/alinaaved/pr-reviewer/internal/http"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://app:app@localhost:5432/app?sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	h := httpapi.NewHandler(db)
	r := chi.NewRouter()
	r.Get("/healthz", h.Healthz)
	r.Post("/team/add", h.TeamAdd)
	r.Get("/team/get", h.TeamGet)
	r.Post("/users/setIsActive", h.UsersSetIsActive)
	r.Get("/users/getReview", h.UsersGetReview)
	r.Post("/pullRequest/create", h.PRCreate)
	r.Post("/pullRequest/merge", h.PRMerge)
	r.Post("/pullRequest/reassign", h.PRReassign)
	r.Get("/stats/assignments-by-user", h.StatsAssignmentsByUser)

	addr := os.Getenv("APP_PORT")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// graceful shutdown
	go func() {
		log.Println("listen", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Println("server stopped")
}
