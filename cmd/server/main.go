package main

import (
	"log"
	"net/http"
	"os"

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

	addr := ":8080"
	if v := os.Getenv("APP_PORT"); v != "" {
		addr = v
	}
	log.Println("listen", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
