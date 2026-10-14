package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/ws", wsHandler)
	log.Println("Arena WS server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
