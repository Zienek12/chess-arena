package sandbox

import (
	"strings"
	"testing"
)

func TestRunArgsApplyIsolationAndLimits(t *testing.T) {
	launcher := NewDockerLauncher()
	args := strings.Join(launcher.runArgs("arena-engine-test", Spec{
		Image:    "example/engine:v1",
		CPUs:     1.5,
		MemoryMB: 256,
	}), " ")
	for _, want := range []string{
		"--network none",
		"--read-only",
		"--cap-drop ALL",
		"--pids-limit 64",
		"--cpus 1.5",
		"--memory 256m --memory-swap 256m",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("missing %q in %q", want, args)
		}
	}
	if !strings.HasSuffix(args, " example/engine:v1") {
		t.Errorf("image must come last: %q", args)
	}
}

func TestRunArgsOmitUnsetLimits(t *testing.T) {
	args := strings.Join(NewDockerLauncher().runArgs("name", Spec{Image: "engine"}), " ")
	if strings.Contains(args, "--cpus") || strings.Contains(args, "--memory") {
		t.Errorf("unexpected limits in %q", args)
	}
}
