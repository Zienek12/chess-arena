package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chess-arena/arena/internal/store"
	"chess-arena/arena/internal/tournament"
)

// fakeStore embeds the interface so tests implement only the methods they use.
type fakeStore struct {
	Store
	createdConfig *tournament.Config
	savedSet      *tournament.PositionSet
	tournamentErr error
}

func (fake *fakeStore) CreateTournament(_ context.Context, config tournament.Config) (store.Tournament, error) {
	fake.createdConfig = &config
	return store.Tournament{ID: "t1", Config: config, Status: tournament.StatusQueued}, fake.tournamentErr
}

func (fake *fakeStore) GetTournament(context.Context, string) (store.TournamentDetail, error) {
	return store.TournamentDetail{}, fake.tournamentErr
}

func (fake *fakeStore) SavePositionSet(_ context.Context, name string, set tournament.PositionSet) (store.PositionSetSummary, bool, error) {
	fake.savedSet = &set
	return store.PositionSetSummary{ID: "p1", SourceName: name, PositionCount: len(set.Positions)}, true, nil
}

func newTestHandler(fake *fakeStore) http.Handler {
	mux := http.NewServeMux()
	NewServer(fake, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	return WithCORS(mux, []string{"http://localhost:3000"})
}

func request(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Origin", "http://localhost:3000")
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestCreateTournamentAppliesDefaults(t *testing.T) {
	fake := &fakeStore{}
	response := request(newTestHandler(fake), "POST", "/api/tournaments",
		`{"name":"cup","engine_a_id":"a","engine_b_id":"b","position_set_id":"p","game_count":10,"limit":{"movetime_ms":1000}}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", response.Code, response.Body)
	}
	if fake.createdConfig.CPUs != tournament.DefaultCPUs || fake.createdConfig.ColorOrder != tournament.ColorAlternate {
		t.Fatalf("defaults not applied: %+v", fake.createdConfig)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatal("CORS header missing for allowed origin")
	}
}

func TestCreateTournamentRejectsInvalidInput(t *testing.T) {
	handler := newTestHandler(&fakeStore{})
	cases := map[string]string{
		"missing limit": `{"name":"cup","engine_a_id":"a","engine_b_id":"b","position_set_id":"p"}`,
		"unknown field": `{"name":"cup","movetime":5}`,
		"broken json":   `{`,
	}
	for name, body := range cases {
		if response := request(handler, "POST", "/api/tournaments", body); response.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d", name, response.Code)
		}
	}
}

func TestStoreErrorsMapToStatusCodes(t *testing.T) {
	cases := map[error]int{
		store.ErrNotFound:         http.StatusNotFound,
		store.ErrInvalidReference: http.StatusUnprocessableEntity,
		io.ErrUnexpectedEOF:       http.StatusInternalServerError,
	}
	for err, status := range cases {
		response := request(newTestHandler(&fakeStore{tournamentErr: err}), "GET", "/api/tournaments/x", "")
		if response.Code != status {
			t.Errorf("%v: got %d, want %d", err, response.Code, status)
		}
	}
}

func TestCreatePositionSetFromPGN(t *testing.T) {
	fake := &fakeStore{}
	response := request(newTestHandler(fake), "POST", "/api/position-sets",
		`{"name":"openings","format":"pgn","content":"[Event \"a\"]\n\n1. e4 e5 *\n\n[Event \"b\"]\n\n1. d4 d5 *\n"}`)
	if response.Code != http.StatusCreated || len(fake.savedSet.Positions) != 2 {
		t.Fatalf("status %d, set %+v", response.Code, fake.savedSet)
	}
}
