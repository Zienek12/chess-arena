package main

import (
  "log"
  "net/http"
  "time"

  "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
  CheckOrigin: func(r *http.Request) bool { return true },
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
  conn, err := upgrader.Upgrade(w, r, nil)
  if err != nil {
    log.Println("upgrade:", err)
    return
  }
  defer conn.Close()

  // simple demo: send a few moves every 2 seconds
  moves := []map[string]string{
    {"type": "move", "from": "e2", "to": "e4"},
    {"type": "move", "from": "e7", "to": "e5"},
    {"type": "move", "from": "g1", "to": "f3"},
    {"type": "move", "from": "b8", "to": "c6"},
  }

  for _, m := range moves {
    if err := conn.WriteJSON(m); err != nil {
      log.Println("write json:", err)
      return
    }
    time.Sleep(2 * time.Second)
  }

  // close message
  conn.WriteJSON(map[string]string{"type": "reset"})
}

func main() {
  http.HandleFunc("/ws", wsHandler)
  log.Println("Arena WS server listening on :8080")
  log.Fatal(http.ListenAndServe(":8080", nil))
}
