// Package play lets a person play against an engine over a WebSocket.
package play

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"chess-arena/arena/internal/sandbox"
)

// LocalVersion runs the engine built in ../engines instead of a Docker image.
const LocalVersion = "local"

// Registry maps engine versions offered to players to Docker images. It is
// configured from the environment and is separate from the tournament engines.
type Registry struct {
	DefaultVersion string
	Images         map[string]string
	LocalPath      string
}

// LoadRegistry reads CHESS_ENGINE_VERSION, CHESS_ENGINE_IMAGES
// ("v1=image:tag,v2=image:tag") and CHESS_ENGINE_PATH.
func LoadRegistry() (Registry, error) {
	registry := Registry{
		DefaultVersion: strings.TrimSpace(os.Getenv("CHESS_ENGINE_VERSION")),
		Images:         make(map[string]string),
		LocalPath:      os.Getenv("CHESS_ENGINE_PATH"),
	}
	if registry.DefaultVersion == "" {
		registry.DefaultVersion = LocalVersion
	}
	if registry.LocalPath == "" {
		executable := "uci_stub"
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

// Versions lists "local" followed by the configured versions in order.
func (registry Registry) Versions() []string {
	versions := make([]string, 0, len(registry.Images))
	for version := range registry.Images {
		versions = append(versions, version)
	}
	sort.Strings(versions)
	return append([]string{LocalVersion}, versions...)
}

// spec returns how to start version and which launcher starts it.
func (registry Registry) spec(version string, docker sandbox.Launcher) (sandbox.Spec, sandbox.Launcher, error) {
	if version == "" {
		version = registry.DefaultVersion
	}
	if image, ok := registry.Images[version]; ok {
		return sandbox.Spec{Image: image, CPUs: 1, MemoryMB: 256}, docker, nil
	}
	if version == LocalVersion {
		return sandbox.Spec{Command: registry.LocalPath}, sandbox.LocalLauncher{}, nil
	}
	return sandbox.Spec{}, nil, fmt.Errorf("engine version %q is not configured", version)
}
