// Package sandbox starts chess engines as isolated processes: Docker containers
// with resource limits in the arena, or plain local processes during development.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// stopGrace is how long an engine may take to exit after its input is closed.
const stopGrace = 3 * time.Second

// Spec describes the engine to start.
type Spec struct {
	Image    string   // Docker image; used by DockerLauncher
	Command  string   // executable path; used by LocalLauncher
	Args     []string // extra arguments for the engine
	CPUs     float64  // CPU limit; 0 means unlimited
	MemoryMB int      // memory limit; 0 means unlimited
}

// Process is a running engine. Close Stdin before calling Stop to let the
// engine exit on its own.
type Process struct {
	Stdin  io.WriteCloser
	Stdout io.Reader
	stop   func() error
}

// NewProcess wraps existing pipes, for example an in-memory engine in tests.
func NewProcess(stdin io.WriteCloser, stdout io.Reader, stop func() error) *Process {
	return &Process{Stdin: stdin, Stdout: stdout, stop: stop}
}

// Stop waits briefly for the engine to exit, then kills it.
func (process *Process) Stop() error {
	return process.stop()
}

// Launcher starts engines.
type Launcher interface {
	Start(ctx context.Context, spec Spec) (*Process, error)
}

// LocalLauncher runs the engine executable directly, without isolation.
type LocalLauncher struct{}

// Start runs spec.Command.
func (LocalLauncher) Start(_ context.Context, spec Spec) (*Process, error) {
	if spec.Command == "" {
		return nil, errors.New("local engine requires a command")
	}
	command := exec.Command(spec.Command, spec.Args...)
	return startCommand(command, func() error { return command.Process.Kill() })
}

// startCommand wires the pipes of command and starts it. kill force-stops the
// engine when it does not exit within stopGrace.
func startCommand(command *exec.Cmd, kill func() error) (*Process, error) {
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open engine stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open engine stdout: %w", err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start engine %q: %w", command.Path, err)
	}

	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	stop := func() error {
		_ = stdin.Close()
		select {
		case err := <-exited:
			return exitError(err)
		case <-time.After(stopGrace):
		}
		killErr := kill()
		<-exited
		if killErr != nil {
			return fmt.Errorf("kill engine: %w", killErr)
		}
		return nil
	}
	return &Process{Stdin: stdin, Stdout: stdout, stop: stop}, nil
}

// exitError ignores non-zero exit codes: engines are stopped deliberately and
// often exit with a signal status.
func exitError(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return nil
	}
	return err
}
