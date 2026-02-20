package main

import (
	"log"
	"net/http"
	"os"

	"novella/internal/api"
	"novella/internal/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/novella.db.json"
	}

	s, err := store.NewWithDB(dbPath)
	if err != nil {
		log.Fatalf("failed to initialize store: %v", err)
	}
	server := api.New(s)

	addr := ":" + port
	log.Printf("novella backend listening on %s (db: %s)", addr, dbPath)
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}
