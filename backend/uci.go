package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type uciEngine struct {
	command *exec.Cmd
	input   io.WriteCloser
	output  *bufio.Scanner
}

func engineCommand(version string, config engineConfig) (string, []string, error) {
	if image, ok := config.Images[version]; ok {
		return "docker", []string{"run", "--rm", "-i", "--init", image}, nil
	}
	if version != "local" && version != "" {
		return "", nil, fmt.Errorf("engine version %q is not configured", version)
	}

	path := os.Getenv("CHESS_ENGINE_PATH")
	if path == "" {
		executable := "uci_stub"
		if runtime.GOOS == "windows" {
			executable += ".exe"
		}
		path = filepath.Join("..", "engines", "target", "debug", executable)
	}
	return path, nil, nil
}

func newUCIEngine() (*uciEngine, error) {
	config, err := loadEngineConfig()
	if err != nil {
		return nil, err
	}
	return newUCIEngineVersion(config.DefaultVersion, config)
}

func newUCIEngineVersion(version string, config engineConfig) (*uciEngine, error) {
	if version == "" {
		version = config.DefaultVersion
	}
	commandPath, args, err := engineCommand(version, config)
	if err != nil {
		return nil, err
	}
	command := exec.Command(commandPath, args...)
	input, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open engine stdin: %w", err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		_ = input.Close()
		return nil, fmt.Errorf("open engine stdout: %w", err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		_ = input.Close()
		_ = output.Close()
		return nil, fmt.Errorf("start engine %q: %w", commandPath, err)
	}

	engine := &uciEngine{
		command: command,
		input:   input,
		output:  bufio.NewScanner(output),
	}
	if err := engine.sendAndWait("uci", "uciok"); err != nil {
		engine.close()
		return nil, err
	}
	if err := engine.sendAndWait("isready", "readyok"); err != nil {
		engine.close()
		return nil, err
	}
	return engine, nil
}

func (engine *uciEngine) send(command string) error {
	_, err := fmt.Fprintf(engine.input, "%s\n", command)
	return err
}

func (engine *uciEngine) sendAndWait(command string, expected string) error {
	if err := engine.send(command); err != nil {
		return fmt.Errorf("send %q: %w", command, err)
	}
	for engine.output.Scan() {
		line := engine.output.Text()
		if strings.HasPrefix(line, "info string ") {
			return fmt.Errorf("engine: %s", strings.TrimPrefix(line, "info string "))
		}
		if line == expected {
			return nil
		}
	}
	if err := engine.output.Err(); err != nil {
		return fmt.Errorf("read engine response: %w", err)
	}
	return fmt.Errorf("engine exited while waiting for %q", expected)
}

func (engine *uciEngine) bestMove() (string, error) {
	if err := engine.send("go"); err != nil {
		return "", fmt.Errorf("send go: %w", err)
	}
	for engine.output.Scan() {
		line := engine.output.Text()
		if strings.HasPrefix(line, "bestmove ") {
			move := strings.TrimSpace(strings.TrimPrefix(line, "bestmove "))
			if move == "" {
				return "", fmt.Errorf("engine returned an empty bestmove")
			}
			return move, nil
		}
	}
	if err := engine.output.Err(); err != nil {
		return "", fmt.Errorf("read bestmove: %w", err)
	}
	return "", fmt.Errorf("engine exited before bestmove")
}

func (engine *uciEngine) close() {
	_ = engine.send("quit")
	_ = engine.input.Close()
	_ = engine.command.Wait()
}
