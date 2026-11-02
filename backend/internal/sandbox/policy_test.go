package sandbox

import (
	"context"
	"strings"
	"testing"
)

func TestRepository(t *testing.T) {
	for image, want := range map[string]string{
		"zienek/chess-arena-engine:1.2.0": "zienek/chess-arena-engine",
		"chess-arena-engine":              "chess-arena-engine",
		"localhost:5000/engine:v1":        "localhost:5000/engine",
		"localhost:5000/engine":           "localhost:5000/engine",
		"zienek/engine@sha256:abc":        "zienek/engine",
		"zienek/engine:1.0@sha256:abc":    "zienek/engine",
		" chess-arena-stockfish:local ":   "chess-arena-stockfish",
	} {
		if got := Repository(image); got != want {
			t.Errorf("Repository(%q) = %q, want %q", image, got, want)
		}
	}
}

func TestImagePolicy(t *testing.T) {
	policy := NewImagePolicy([]string{"zienek/*", "chess-arena-stockfish", " "})
	for image, allowed := range map[string]bool{
		"zienek/chess-arena-engine:1.2.0": true,
		"zienek/anything":                 true,
		"chess-arena-stockfish:local":     true,
		"chess-arena-stockfish-evil":      false,
		"zienekfake/engine":               false,
		"alpine":                          false,
	} {
		if got := policy.Allows(image); got != allowed {
			t.Errorf("Allows(%q) = %v, want %v", image, got, allowed)
		}
	}
	if err := policy.Check("alpine"); err == nil || !strings.Contains(err.Error(), "zienek/*") {
		t.Fatalf("the error must list the allowed images: %v", err)
	}
	if !NewImagePolicy([]string{"*"}).Allows("alpine") {
		t.Fatal(`"*" allows every image`)
	}
	if NewImagePolicy(nil).Allows("alpine") {
		t.Fatal("an empty policy allows nothing")
	}
}

func TestDockerLauncherRejectsImagesOutsideThePolicy(t *testing.T) {
	policy := NewImagePolicy([]string{"zienek/*"})
	launcher := &DockerLauncher{Binary: "docker-must-not-run", Policy: &policy}
	if _, err := launcher.Start(context.Background(), Spec{Image: "alpine"}); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected a policy error, got %v", err)
	}
}
