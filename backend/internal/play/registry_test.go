package play

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"chess-arena/arena/internal/store"
)

type fakeEngines struct {
	engines []store.Engine
	err     error
}

func (source fakeEngines) ListEngines(context.Context) ([]store.Engine, error) {
	return source.engines, source.err
}

func TestChoicesCombineRegisteredAndConfiguredEngines(t *testing.T) {
	local := filepath.Join(t.TempDir(), "engine")
	if err := os.WriteFile(local, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	registry := Registry{
		Images: map[string]string{
			"random":      "chess-arena-engine:random",
			"Alpha 1.2.0": "duplicate:image",
		},
		LocalPath: local,
	}
	source := fakeEngines{engines: []store.Engine{
		{Name: "Alpha", Version: "1.2.0", Image: "zienek/chess-arena-engine:1.2.0", UCIOptions: map[string]string{"Hash": "16"}},
		{Name: "Alpha", Version: "1.0.0", Image: "zienek/chess-arena-engine:engine-v1.0"},
	}}

	choices, err := registry.Choices(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(choices), []string{"Alpha 1.2.0", "Alpha 1.0.0", "random", "local"}; !slices.Equal(got, want) {
		t.Fatalf("choices = %v, want %v", got, want)
	}
	if choices[0].Spec.Image != "zienek/chess-arena-engine:1.2.0" || choices[0].UCIOptions["Hash"] != "16" {
		t.Fatalf("registered engine was not used: %+v", choices[0])
	}
	if !choices[3].Local || choices[3].Spec.Command != local {
		t.Fatalf("unexpected local choice %+v", choices[3])
	}
}

func TestChoicesHideMissingLocalBinary(t *testing.T) {
	registry := Registry{LocalPath: filepath.Join(t.TempDir(), "missing")}
	choices, err := registry.Choices(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(choices) != 0 {
		t.Fatalf("choices = %v, want none", names(choices))
	}
}

func TestChoicesReportSourceErrors(t *testing.T) {
	failure := errors.New("database is down")
	if _, err := (Registry{}).Choices(context.Background(), fakeEngines{err: failure}); !errors.Is(err, failure) {
		t.Fatalf("err = %v, want %v", err, failure)
	}
}

func TestDefaultPrefersConfiguredVersion(t *testing.T) {
	choices := []Choice{{Name: "Alpha 1.2.0"}, {Name: "random"}}
	if got := (Registry{DefaultVersion: "random"}).Default(choices); got != "random" {
		t.Fatalf("default = %q, want random", got)
	}
	if got := (Registry{DefaultVersion: "gone"}).Default(choices); got != "Alpha 1.2.0" {
		t.Fatalf("default = %q, want the first choice", got)
	}
	if got := (Registry{}).Default(nil); got != "" {
		t.Fatalf("default = %q, want empty", got)
	}
}
