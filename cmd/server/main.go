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

	s := store.New()
	server := api.New(s)

	addr := ":" + port
	log.Printf("novella backend listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}
