package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chess-arena/arena/internal/sandbox"
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
	return newTestHandlerWith(fake, Options{})
}

func newTestHandlerWith(fake *fakeStore, options Options) http.Handler {
	mux := http.NewServeMux()
	NewServer(fake, slog.New(slog.NewTextHandler(io.Discard, nil)), options).Register(mux)
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

func (fake *fakeStore) CreateEngine(_ context.Context, input store.NewEngine) (store.Engine, error) {
	return store.Engine{ID: "e1", Name: input.Name, Version: input.Version, Image: input.Image}, nil
}

func TestWritesRequireTheToken(t *testing.T) {
	handler := newTestHandlerWith(&fakeStore{}, Options{Token: "secret"})
	body := `{"name":"engine","version":"1","image":"engine:1"}`

	if response := request(handler, "POST", "/api/engines", body); response.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: status %d", response.Code)
	}
	wrong := httptest.NewRequest("POST", "/api/engines", strings.NewReader(body))
	wrong.Header.Set("Authorization", "Bearer guess")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, wrong)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token: status %d", recorder.Code)
	}

	valid := httptest.NewRequest("POST", "/api/engines", strings.NewReader(body))
	valid.Header.Set("Authorization", "Bearer secret")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, valid)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("valid token: status %d: %s", recorder.Code, recorder.Body)
	}

	// Reads stay public, and the frontend can ask whether a token is needed.
	status := request(handler, "GET", "/api/auth", "")
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"token_required":true,"authorized":false`) {
		t.Fatalf("auth status: %d %s", status.Code, status.Body)
	}
}

func TestEngineImagesMustBeAllowed(t *testing.T) {
	policy := sandbox.NewImagePolicy([]string{"zienek/*"})
	handler := newTestHandlerWith(&fakeStore{}, Options{Images: &policy})
	response := request(handler, "POST", "/api/engines", `{"name":"evil","version":"1","image":"alpine:latest"}`)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "not allowed") {
		t.Fatalf("status %d: %s", response.Code, response.Body)
	}
	response = request(handler, "POST", "/api/engines", `{"name":"ok","version":"1","image":"zienek/chess-arena-engine:1.2.0"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", response.Code, response.Body)
	}
}

type fakeEvents struct{ events chan store.Event }

func (fake fakeEvents) Subscribe() (<-chan store.Event, func()) { return fake.events, func() {} }

func TestEventStreamFiltersByTournament(t *testing.T) {
	stream := make(chan store.Event, 3)
	stream <- store.Event{Type: store.EventMove, TournamentID: "other", GameID: "g0"}
	stream <- store.Event{Type: store.EventMove, TournamentID: "t1", GameID: "g1"}
	close(stream)
	handler := newTestHandlerWith(&fakeStore{}, Options{Events: fakeEvents{events: stream}})

	response := request(handler, "GET", "/api/events?tournament_id=t1", "")
	body := response.Body.String()
	if response.Header().Get("Content-Type") != "text/event-stream" || !strings.HasPrefix(body, "retry: 3000") {
		t.Fatalf("not an event stream: %q", body)
	}
	if strings.Contains(body, "g0") || !strings.Contains(body, `data: {"type":"move","tournament_id":"t1","game_id":"g1"}`) {
		t.Fatalf("unexpected events: %q", body)
	}
}
