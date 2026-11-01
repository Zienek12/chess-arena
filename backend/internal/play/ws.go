package play

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/gorilla/websocket"

	"chess-arena/arena/internal/rules"
	"chess-arena/arena/internal/sandbox"
	"chess-arena/arena/internal/uci"
)

const (
	engineMoveTime   = time.Second
	engineMoveMargin = 5 * time.Second
	startTimeout     = 30 * time.Second
)

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
	Result    string   `json:"result,omitempty"`
}

// Handler serves one human-versus-engine game per WebSocket connection.
type Handler struct {
	registry Registry
	docker   sandbox.Launcher
	origins  []string
	logger   *slog.Logger
}

// NewHandler returns the WebSocket handler. Connections are accepted only from
// the listed browser origins.
func NewHandler(registry Registry, docker sandbox.Launcher, origins []string, logger *slog.Logger) *Handler {
	return &Handler{registry: registry, docker: docker, origins: origins, logger: logger}
}

// session is the state of one connection.
type session struct {
	handler *Handler
	conn    *websocket.Conn
	engine  *uci.Client
	version string
	game    *rules.Game
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || slices.Contains(handler.origins, origin)
	}}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		handler.logger.Warn("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	ctx := r.Context()
	s := &session{handler: handler, conn: conn}
	defer s.closeEngine()
	if err := s.reset(ctx, handler.registry.DefaultVersion); err != nil {
		_ = conn.WriteJSON(serverMessage{Type: "error", Message: err.Error()})
		return
	}
	for {
		var message clientMessage
		if err := conn.ReadJSON(&message); err != nil {
			if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				handler.logger.Warn("websocket read failed", "error", err)
			}
			return
		}
		switch message.Type {
		case "reset":
			version := s.version
			if message.Engine != "" {
				version = message.Engine
			}
			if err := s.reset(ctx, version); err != nil {
				_ = conn.WriteJSON(serverMessage{Type: "error", Message: err.Error()})
				return
			}
		case "move":
			if err := s.move(ctx, message); err != nil {
				return
			}
		default:
			_ = conn.WriteJSON(serverMessage{Type: "error", Message: "unknown message type"})
		}
	}
}

// reset starts a fresh engine of version and a new game.
func (s *session) reset(ctx context.Context, version string) error {
	s.closeEngine()
	spec, launcher, err := s.handler.registry.spec(version, s.handler.docker)
	if err != nil {
		return err
	}
	process, err := launcher.Start(ctx, spec)
	if err != nil {
		return err
	}
	engine := uci.NewClient(process.Stdin, process.Stdout, process.Stop)
	if _, err := engine.Handshake(ctx, nil, startTimeout); err != nil {
		_ = engine.Close()
		return err
	}
	s.engine, s.version = engine, version
	s.game, _ = rules.NewGame("")
	return s.conn.WriteJSON(serverMessage{
		Type: "state", Moves: []string{}, Turn: string(rules.White),
		Engine: version, Engines: s.handler.registry.Versions(),
	})
}

// move applies the player's move and answers with the engine's move. Rejected
// moves are reported to the client; only connection failures are returned.
func (s *session) move(ctx context.Context, message clientMessage) error {
	if message.From == "" || message.To == "" {
		return s.conn.WriteJSON(serverMessage{Type: "error", Message: "move requires from and to squares"})
	}
	if err := s.game.Play(message.From + message.To + message.Promotion); err != nil {
		return s.conn.WriteJSON(serverMessage{Type: "error", Message: err.Error()})
	}
	if err := s.conn.WriteJSON(serverMessage{
		Type: "move", From: message.From, To: message.To, Promotion: message.Promotion,
		Turn: string(s.game.Turn()),
	}); err != nil {
		return err
	}
	if s.game.Over() {
		return s.writeGameOver()
	}

	result, err := s.engine.Search(ctx, uci.Position{Moves: s.game.Moves()},
		uci.Limit{MoveTime: engineMoveTime}, engineMoveTime+engineMoveMargin)
	if err == nil {
		err = s.game.Play(result.BestMove)
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		return s.conn.WriteJSON(serverMessage{Type: "error", Message: fmt.Sprintf("engine failed: %v", err)})
	}
	if err := s.conn.WriteJSON(serverMessage{
		Type: "move", From: result.BestMove[:2], To: result.BestMove[2:4], Promotion: result.BestMove[4:],
		Turn: string(s.game.Turn()),
	}); err != nil {
		return err
	}
	if s.game.Over() {
		return s.writeGameOver()
	}
	return nil
}

func (s *session) writeGameOver() error {
	return s.conn.WriteJSON(serverMessage{
		Type: "game_over", Turn: string(s.game.Turn()), Result: s.game.Result(),
		Message: fmt.Sprintf("%s by %s", s.game.Result(), s.game.Termination()),
	})
}

func (s *session) closeEngine() {
	if s.engine != nil {
		_ = s.engine.Close()
		s.engine = nil
	}
}
