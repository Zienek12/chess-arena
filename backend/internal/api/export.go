package api

import (
	"context"
	"encoding/csv"
	"net/http"

	"chess-arena/arena/internal/store"
)

// exportTable serves a tournament's table as a CSV download.
func (server *Server) exportTable(name string, load func(context.Context, string) (store.Table, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, err := server.store.GetTournament(r.Context(), id); err != nil {
			server.writeStoreError(w, r, err)
			return
		}
		table, err := load(r.Context(), id)
		if err != nil {
			server.writeStoreError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="tournament-`+id+`-`+name+`.csv"`)
		writer := csv.NewWriter(w)
		_ = writer.Write(table.Header)
		_ = writer.WriteAll(table.Rows)
	}
}

func (server *Server) exportPGN(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := server.store.GetTournament(r.Context(), id); err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	pgn, err := server.store.TournamentPGN(r.Context(), id)
	if err != nil {
		server.writeStoreError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/x-chess-pgn")
	w.Header().Set("Content-Disposition", `attachment; filename="tournament-`+id+`.pgn"`)
	_, _ = w.Write([]byte(pgn))
}
