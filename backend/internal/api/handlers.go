package api

import (
	"net/http"
	"strings"

	"chess-arena/arena/internal/store"
	"chess-arena/arena/internal/tournament"
)

func (server *Server) listEngines(w http.ResponseWriter, r *http.Request) {
	engines, err := server.store.ListEngines(r.Context())
	if err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, engines)
}

func (server *Server) createEngine(w http.ResponseWriter, r *http.Request) {
	var input store.NewEngine
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := input.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	engine, err := server.store.CreateEngine(r.Context(), input)
	if err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, engine)
}

type positionSetRequest struct {
	Name    string `json:"name"`
	Format  string `json:"format"` // "fen" or "pgn"
	Content string `json:"content"`
}

func (server *Server) listPositionSets(w http.ResponseWriter, r *http.Request) {
	sets, err := server.store.ListPositionSets(r.Context())
	if err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sets)
}

func (server *Server) createPositionSet(w http.ResponseWriter, r *http.Request) {
	var input positionSetRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Name) == "" {
		writeError(w, http.StatusBadRequest, "position set name is required")
		return
	}
	var set tournament.PositionSet
	var err error
	switch input.Format {
	case string(tournament.PositionSourceFEN):
		set, err = tournament.ImportFEN(input.Content)
	case string(tournament.PositionSourcePGN):
		set, err = tournament.ImportPGN(input.Content)
	default:
		writeError(w, http.StatusBadRequest, `format must be "fen" or "pgn"`)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	summary, created, err := server.store.SavePositionSet(r.Context(), strings.TrimSpace(input.Name), set)
	if err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	writeJSON(w, status, summary)
}

func (server *Server) listTournaments(w http.ResponseWriter, r *http.Request) {
	tournaments, err := server.store.ListTournaments(r.Context())
	if err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tournaments)
}

func (server *Server) createTournament(w http.ResponseWriter, r *http.Request) {
	var config tournament.Config
	if !decodeJSON(w, r, &config) {
		return
	}
	config = config.Normalize()
	if err := config.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := server.store.CreateTournament(r.Context(), config)
	if err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (server *Server) getTournament(w http.ResponseWriter, r *http.Request) {
	detail, err := server.store.GetTournament(r.Context(), r.PathValue("id"))
	if err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (server *Server) cancelTournament(w http.ResponseWriter, r *http.Request) {
	if err := server.store.CancelTournament(r.Context(), r.PathValue("id")); err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (server *Server) listGames(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := server.store.GetTournament(r.Context(), id); err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	games, err := server.store.ListGames(r.Context(), id)
	if err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, games)
}

func (server *Server) getGame(w http.ResponseWriter, r *http.Request) {
	game, err := server.store.GetGame(r.Context(), r.PathValue("id"))
	if err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, game)
}

func (server *Server) getGamePGN(w http.ResponseWriter, r *http.Request) {
	game, err := server.store.GetGame(r.Context(), r.PathValue("id"))
	if err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	if game.PGN == "" {
		writeError(w, http.StatusConflict, "game is not finished")
		return
	}
	w.Header().Set("Content-Type", "application/x-chess-pgn")
	w.Header().Set("Content-Disposition", `attachment; filename="game-`+game.ID+`.pgn"`)
	_, _ = w.Write([]byte(game.PGN))
}
