package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type clientMessage struct {
	Type      string `json:"type"`
	From      string `json:"from"`
	To        string `json:"to"`
	Promotion string `json:"promotion,omitempty"`
	Engine    string `json:"engine,omitempty"`
}

type serverMessage struct {
	Type      string   `json:"type"`
	From      string   `json:"from,omitempty"`
	To        string   `json:"to,omitempty"`
	Promotion string   `json:"promotion,omitempty"`
	Moves     []string `json:"moves,omitempty"`
	Turn      string   `json:"turn,omitempty"`
	Message   string   `json:"message,omitempty"`
	Engine    string   `json:"engine,omitempty"`
	Engines   []string `json:"engines,omitempty"`
}

func writeState(conn *websocket.Conn, history []string, engine string, config engineConfig) error {
	return conn.WriteJSON(serverMessage{
		Type: "state", Moves: history, Turn: "w", Engine: engine, Engines: config.versions(),
	})
}

func parseUCIMove(move string) (string, string, string, error) {
	if len(move) != 4 && len(move) != 5 {
		return "", "", "", fmt.Errorf("invalid engine move %q", move)
	}
	if move[0] < 'a' || move[0] > 'h' || move[2] < 'a' || move[2] > 'h' ||
		move[1] < '1' || move[1] > '8' || move[3] < '1' || move[3] > '8' {
		return "", "", "", fmt.Errorf("invalid engine move %q", move)
	}
	promotion := ""
	if len(move) == 5 {
		promotion = move[4:]
	}
	return move[:2], move[2:4], promotion, nil
}

func appendPlayerMove(engine *uciEngine, history []string, message clientMessage) ([]string, error) {
	if message.From == "" || message.To == "" {
		return history, fmt.Errorf("move requires from and to squares")
	}
	move := message.From + message.To + message.Promotion
	candidate := append(append([]string(nil), history...), move)
	if err := setEnginePosition(engine, candidate); err != nil {
		return history, err
	}
	return candidate, nil
}

func setEnginePosition(engine *uciEngine, history []string) error {
	command := "position startpos"
	if len(history) > 0 {
		command += " moves " + strings.Join(history, " ")
	}
	if err := engine.send(command); err != nil {
		return fmt.Errorf("send position: %w", err)
	}
	if err := engine.send("isready"); err != nil {
		return fmt.Errorf("send isready: %w", err)
	}
	var positionError error
	for engine.output.Scan() {
		line := engine.output.Text()
		if strings.HasPrefix(line, "info string ") {
			positionError = fmt.Errorf("engine: %s", strings.TrimPrefix(line, "info string "))
			continue
		}
		if line == "readyok" {
			return positionError
		}
	}
	if err := engine.output.Err(); err != nil {
		return fmt.Errorf("read position response: %w", err)
	}
	return fmt.Errorf("engine exited while applying position")
}

func resetEngine(engine *uciEngine, version string, config engineConfig) (*uciEngine, []string, error) {
	engine.close()
	next, err := newUCIEngineVersion(version, config)
	if err != nil {
		return nil, nil, err
	}
	return next, make([]string, 0, 256), nil
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	defer conn.Close()

	config, err := loadEngineConfig()
	if err != nil {
		_ = conn.WriteJSON(serverMessage{Type: "error", Message: err.Error()})
		return
	}
	selectedEngine := config.DefaultVersion
	engine, err := newUCIEngineVersion(selectedEngine, config)
	if err != nil {
		_ = conn.WriteJSON(serverMessage{Type: "error", Message: err.Error()})
		return
	}
	defer engine.close()

	history := make([]string, 0, 256)
	if err := writeState(conn, history, selectedEngine, config); err != nil {
		log.Println("write initial state:", err)
		return
	}

	for {
		var message clientMessage
		if err := conn.ReadJSON(&message); err != nil {
			if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Println("read json:", err)
			}
			return
		}

		switch message.Type {
		case "reset":
			if message.Engine != "" {
				selectedEngine = message.Engine
			}
			engine, history, err = resetEngine(engine, selectedEngine, config)
			if err != nil {
				_ = conn.WriteJSON(serverMessage{Type: "error", Message: err.Error()})
				return
			}
			if err := writeState(conn, history, selectedEngine, config); err != nil {
				return
			}
		case "move":
			nextHistory, err := appendPlayerMove(engine, history, message)
			if err != nil {
				_ = conn.WriteJSON(serverMessage{Type: "error", Message: err.Error()})
				continue
			}
			history = nextHistory
			if err := conn.WriteJSON(serverMessage{
				Type:      "move",
				From:      message.From,
				To:        message.To,
				Promotion: message.Promotion,
				Turn:      "b",
			}); err != nil {
				return
			}

			engineMove, err := engine.bestMove()
			if err != nil {
				_ = conn.WriteJSON(serverMessage{Type: "error", Message: err.Error()})
				continue
			}
			if engineMove == "0000" {
				_ = conn.WriteJSON(serverMessage{Type: "game_over", Turn: "b"})
				continue
			}
			from, to, promotion, err := parseUCIMove(engineMove)
			if err != nil {
				_ = conn.WriteJSON(serverMessage{Type: "error", Message: err.Error()})
				continue
			}
			history = append(history, engineMove)
			if err := conn.WriteJSON(serverMessage{
				Type:      "move",
				From:      from,
				To:        to,
				Promotion: promotion,
				Turn:      "w",
			}); err != nil {
				return
			}
		default:
			_ = conn.WriteJSON(serverMessage{Type: "error", Message: "unknown message type"})
		}
	}
}
