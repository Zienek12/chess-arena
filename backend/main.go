package main

import (
	"context"
	"log"
	"net/http"
	"os"
)

func main() {
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		store, err := OpenTournamentStore(context.Background(), databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
		if err := store.ApplyMigrations(context.Background()); err != nil {
			log.Fatal(err)
		}
		log.Println("Tournament database schema is ready")
	}

	http.HandleFunc("/ws", wsHandler)
	log.Println("Arena WS server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
