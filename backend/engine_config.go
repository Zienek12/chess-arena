package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

type engineConfig struct {
	DefaultVersion string
	Images         map[string]string
}

func loadEngineConfig() (engineConfig, error) {
	config := engineConfig{
		DefaultVersion: strings.TrimSpace(os.Getenv("CHESS_ENGINE_VERSION")),
		Images:         make(map[string]string),
	}
	if config.DefaultVersion == "" {
		config.DefaultVersion = "local"
	}

	for _, entry := range strings.Split(os.Getenv("CHESS_ENGINE_IMAGES"), ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return engineConfig{}, fmt.Errorf("invalid CHESS_ENGINE_IMAGES entry %q; expected version=image", entry)
		}
		config.Images[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return config, nil
}

func (config engineConfig) versions() []string {
	versions := make([]string, 0, len(config.Images)+1)
	versions = append(versions, "local")
	for version := range config.Images {
		versions = append(versions, version)
	}
	sort.Strings(versions[1:])
	return versions
}
