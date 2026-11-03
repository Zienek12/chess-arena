// Package api exposes engines, position sets, tournaments and games over REST.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"chess-arena/arena/internal/sandbox"
	"chess-arena/arena/internal/store"
	"chess-arena/arena/internal/tournament"
)

// maxBodyBytes limits request bodies; position sets are the largest payloads.
const maxBodyBytes = 10 << 20

// Store is the persistence the API needs.
type Store interface {
	CreateEngine(ctx context.Context, engine store.NewEngine) (store.Engine, error)
	ListEngines(ctx context.Context) ([]store.Engine, error)
	SavePositionSet(ctx context.Context, name string, set tournament.PositionSet) (store.PositionSetSummary, bool, error)
	ListPositionSets(ctx context.Context) ([]store.PositionSetSummary, error)
	CreateTournament(ctx context.Context, config tournament.Config) (store.Tournament, error)
	ListTournaments(ctx context.Context) ([]store.Tournament, error)
	GetTournament(ctx context.Context, id string) (store.TournamentDetail, error)
	CancelTournament(ctx context.Context, id string) error
	ListGames(ctx context.Context, tournamentID string) ([]store.GameSummary, error)
	GetGame(ctx context.Context, id string) (store.GameDetail, error)
	Leaderboard(ctx context.Context) ([]store.LeaderboardEntry, error)
	ExportGames(ctx context.Context, tournamentID string) (store.Table, error)
	ExportMoves(ctx context.Context, tournamentID string) (store.Table, error)
	TournamentPGN(ctx context.Context, tournamentID string) (string, error)
}

// Options configure optional API features.
type Options struct {
	// Token, when set, is required as "Authorization: Bearer <token>" for
	// every request that changes data.
	Token string
	// Images, when set, limits the Docker images engines may be registered with.
	Images *sandbox.ImagePolicy
	// Events, when set, enables the /api/events stream.
	Events Events
}

// Server holds the API dependencies.
type Server struct {
	store   Store
	logger  *slog.Logger
	options Options
}

// NewServer returns an API server.
func NewServer(store Store, logger *slog.Logger, options Options) *Server {
	return &Server{store: store, logger: logger, options: options}
}

// Register adds the API routes to mux.
func (server *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/auth", server.authStatus)
	mux.HandleFunc("GET /api/engines", server.listEngines)
	mux.HandleFunc("POST /api/engines", server.requireToken(server.createEngine))
	mux.HandleFunc("GET /api/position-sets", server.listPositionSets)
	mux.HandleFunc("POST /api/position-sets", server.requireToken(server.createPositionSet))
	mux.HandleFunc("GET /api/tournaments", server.listTournaments)
	mux.HandleFunc("POST /api/tournaments", server.requireToken(server.createTournament))
	mux.HandleFunc("GET /api/tournaments/{id}", server.getTournament)
	mux.HandleFunc("POST /api/tournaments/{id}/cancel", server.requireToken(server.cancelTournament))
	mux.HandleFunc("GET /api/tournaments/{id}/games", server.listGames)
	mux.HandleFunc("GET /api/tournaments/{id}/export/games.csv", server.exportTable("games", server.store.ExportGames))
	mux.HandleFunc("GET /api/tournaments/{id}/export/moves.csv", server.exportTable("moves", server.store.ExportMoves))
	mux.HandleFunc("GET /api/tournaments/{id}/export/games.pgn", server.exportPGN)
	mux.HandleFunc("GET /api/games/{id}", server.getGame)
	mux.HandleFunc("GET /api/games/{id}/pgn", server.getGamePGN)
	mux.HandleFunc("GET /api/leaderboard", server.leaderboard)
	if server.options.Events != nil {
		mux.HandleFunc("GET /api/events", server.streamEvents)
	}
}

// WithCORS allows the listed browser origins to call the API.
func WithCORS(next http.Handler, origins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && slices.Contains(origins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

// writeStoreError maps store errors to HTTP statuses and hides internal details.
func (server *Server) writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, store.ErrInvalidReference):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		server.logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

// decodeJSON reads a JSON body, rejecting unknown fields to catch typos.
func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}
