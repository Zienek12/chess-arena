// Package play lets a person play against an engine over a WebSocket.
package play

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"chess-arena/arena/internal/sandbox"
	"chess-arena/arena/internal/store"
)

// LocalVersion runs the engine built in ../engines instead of a Docker image.
const LocalVersion = "local"

// EngineSource lists the engines registered for tournaments. *store.Store
// implements it.
type EngineSource interface {
	ListEngines(ctx context.Context) ([]store.Engine, error)
}

// Registry holds the engines configured from the environment. Players can
// choose these and every engine registered for tournaments.
type Registry struct {
	DefaultVersion string
	Images         map[string]string
	LocalPath      string
}

// Choice is one engine a player can select.
type Choice struct {
	Name       string
	Spec       sandbox.Spec
	UCIOptions map[string]string
	Local      bool
}

// LoadRegistry reads CHESS_ENGINE_VERSION, CHESS_ENGINE_IMAGES
// ("v1=image:tag,v2=image:tag") and CHESS_ENGINE_PATH.
func LoadRegistry() (Registry, error) {
	registry := Registry{
		DefaultVersion: strings.TrimSpace(os.Getenv("CHESS_ENGINE_VERSION")),
		Images:         make(map[string]string),
		LocalPath:      os.Getenv("CHESS_ENGINE_PATH"),
	}
	if registry.LocalPath == "" {
		executable := "chess-arena-engine"
		if runtime.GOOS == "windows" {
			executable += ".exe"
		}
		registry.LocalPath = filepath.Join("..", "engines", "target", "debug", executable)
	}
	for _, entry := range strings.Split(os.Getenv("CHESS_ENGINE_IMAGES"), ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		version, image, found := strings.Cut(entry, "=")
		version, image = strings.TrimSpace(version), strings.TrimSpace(image)
		if !found || version == "" || image == "" {
			return Registry{}, fmt.Errorf("invalid CHESS_ENGINE_IMAGES entry %q; expected version=image", entry)
		}
		registry.Images[version] = image
	}
	return registry, nil
}

// Choices lists the registered engines ("name version", in registry order),
// then the configured images by version, then "local" when its binary exists.
// source may be nil when there is no database.
func (registry Registry) Choices(ctx context.Context, source EngineSource) ([]Choice, error) {
	var choices []Choice
	seen := make(map[string]bool)
	add := func(choice Choice) {
		if !seen[choice.Name] {
			seen[choice.Name] = true
			choices = append(choices, choice)
		}
	}

	if source != nil {
		engines, err := source.ListEngines(ctx)
		if err != nil {
			return nil, err
		}
		for _, engine := range engines {
			add(Choice{Name: engine.DisplayName(), Spec: dockerSpec(engine.Image), UCIOptions: engine.UCIOptions})
		}
	}

	versions := make([]string, 0, len(registry.Images))
	for version := range registry.Images {
		versions = append(versions, version)
	}
	sort.Strings(versions)
	for _, version := range versions {
		add(Choice{Name: version, Spec: dockerSpec(registry.Images[version])})
	}

	if info, err := os.Stat(registry.LocalPath); err == nil && !info.IsDir() {
		add(Choice{Name: LocalVersion, Spec: sandbox.Spec{Command: registry.LocalPath}, Local: true})
	}
	return choices, nil
}

// Default is the engine selected when a player connects: CHESS_ENGINE_VERSION
// when it is available, otherwise the first choice.
func (registry Registry) Default(choices []Choice) string {
	for _, choice := range choices {
		if choice.Name == registry.DefaultVersion {
			return choice.Name
		}
	}
	if len(choices) == 0 {
		return ""
	}
	return choices[0].Name
}

func dockerSpec(image string) sandbox.Spec {
	return sandbox.Spec{Image: image, CPUs: 1, MemoryMB: 256}
}

// names returns the name of every choice.
func names(choices []Choice) []string {
	result := make([]string, len(choices))
	for index, choice := range choices {
		result[index] = choice.Name
	}
	return result
}

// find returns the choice called name.
func find(choices []Choice, name string) (Choice, error) {
	for _, choice := range choices {
		if choice.Name == name {
			return choice, nil
		}
	}
	return Choice{}, fmt.Errorf("engine %q is not available", name)
}
