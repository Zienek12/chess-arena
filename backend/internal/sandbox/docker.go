package sandbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DockerLauncher runs every engine in its own container through the Docker CLI.
// Containers have no network, a read-only filesystem, no capabilities, and the
// CPU and memory limits from the Spec, so one engine cannot starve another.
type DockerLauncher struct {
	Binary    string // docker executable
	Network   string // container network; "none" isolates the engine
	PidsLimit int
	// Policy, when set, lists the images that may be started.
	Policy *ImagePolicy
}

// NewDockerLauncher returns a launcher with isolated defaults.
func NewDockerLauncher() *DockerLauncher {
	return &DockerLauncher{Binary: "docker", Network: "none", PidsLimit: 64}
}

// Start pulls the image when it is missing and starts a container. The
// container runs the image by its ID, so the tag cannot be moved to another
// build in between, and the ID is reported in Process.ImageID.
func (launcher *DockerLauncher) Start(ctx context.Context, spec Spec) (*Process, error) {
	if spec.Image == "" {
		return nil, errors.New("docker engine requires an image")
	}
	if launcher.Policy != nil {
		if err := launcher.Policy.Check(spec.Image); err != nil {
			return nil, err
		}
	}
	imageID, err := launcher.ImageID(ctx, spec.Image)
	if err != nil {
		return nil, err
	}
	name, err := containerName()
	if err != nil {
		return nil, err
	}
	pinned := spec
	pinned.Image = imageID
	command := exec.Command(launcher.Binary, launcher.runArgs(name, pinned)...)
	kill := func() error {
		killCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return exec.CommandContext(killCtx, launcher.Binary, "kill", name).Run()
	}
	process, err := startCommand(command, kill)
	if err != nil {
		return nil, err
	}
	process.ImageID = imageID
	return process, nil
}

// ImageID returns the ID of image, pulling it first when it is not present.
func (launcher *DockerLauncher) ImageID(ctx context.Context, image string) (string, error) {
	if id, err := launcher.inspectImage(ctx, image); err == nil {
		return id, nil
	}
	var output bytes.Buffer
	pull := exec.CommandContext(ctx, launcher.Binary, "pull", image)
	pull.Stdout = &output
	pull.Stderr = &output
	if err := pull.Run(); err != nil {
		return "", fmt.Errorf("pull image %s: %w: %s", image, err, strings.TrimSpace(output.String()))
	}
	return launcher.inspectImage(ctx, image)
}

func (launcher *DockerLauncher) inspectImage(ctx context.Context, image string) (string, error) {
	output, err := exec.CommandContext(ctx, launcher.Binary, "image", "inspect", "--format", "{{.Id}}", image).Output()
	if err != nil {
		return "", fmt.Errorf("inspect image %s: %w", image, err)
	}
	id := strings.TrimSpace(string(output))
	if id == "" {
		return "", fmt.Errorf("inspect image %s: empty ID", image)
	}
	return id, nil
}

func (launcher *DockerLauncher) runArgs(name string, spec Spec) []string {
	args := []string{
		"run", "--rm", "--interactive", "--init",
		"--name", name,
		"--label", "chess-arena.role=engine",
		"--network", launcher.Network,
		"--read-only",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
	}
	if launcher.PidsLimit > 0 {
		args = append(args, "--pids-limit", strconv.Itoa(launcher.PidsLimit))
	}
	if spec.CPUs > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(spec.CPUs, 'f', -1, 64))
	}
	if spec.MemoryMB > 0 {
		memory := strconv.Itoa(spec.MemoryMB) + "m"
		// Equal swap limit disables swap, so the memory limit is a hard limit.
		args = append(args, "--memory", memory, "--memory-swap", memory)
	}
	args = append(args, spec.Image)
	return append(args, spec.Args...)
}

func containerName() (string, error) {
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("generate container name: %w", err)
	}
	return "arena-engine-" + hex.EncodeToString(suffix), nil
}
