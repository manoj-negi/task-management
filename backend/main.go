package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"

	"office-task-manager/backend/internal/config"
	"office-task-manager/backend/internal/database"
	"office-task-manager/backend/internal/routes"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found; using environment variables")
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := database.EnsureTaskSoftDeleteSchema(db); err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           routes.New(db),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Office Task Manager API running at http://localhost:%s", cfg.Port)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
