package pirate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

const tickInterval = 10 * time.Second

func runScript(ctx context.Context, fname, contents string, env map[string]string, l *slog.Logger, isBinary bool) error {
	name, err := writeTmpFile(l, fname, contents)
	if err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer cleanupFile(l, name)

	cmd := exec.CommandContext(runCtx, "bash", name)

	for key, val := range env {
		value := val

		if key == "PIRATE_BODY" && isBinary {
			binName, err := writeTmpFile(l, "pirate-webhook-binary-*", val)
			if err != nil {
				return fmt.Errorf("could not write body: %w", err)
			}

			defer cleanupFile(l, binName)

			value = binName
		}

		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	stdout, stderr := newSafeBuffer(), newSafeBuffer()

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start command: %w", err)
	}

	ticker := time.NewTicker(tickInterval)

	go func() {
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				flush(stdout, stderr, l)
			}
		}
	}()

	defer flush(stdout, stderr, l)
	if err := cmd.Wait(); err != nil {
		code := 1

		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		}

		return fmt.Errorf("failed (exit code=%d): %w", code, err)
	}

	return nil
}

func writeTmpFile(l *slog.Logger, fname string, contents string) (string, error) {
	fd, err := os.CreateTemp("", fname)
	if err != nil {
		return "", fmt.Errorf("could not create script: %w", err)
	}
	name := fd.Name()

	wroteBytes, err := fd.WriteString(contents)
	if err != nil {
		cleanupFile(l, name)
		return name, fmt.Errorf("error writing script: %w", err)
	}

	wantBytes := len([]byte(contents))

	if wantBytes != wroteBytes {
		return "", fmt.Errorf("expected %d bytes, wrote %d", wantBytes, wroteBytes)
	}

	fd.Close()

	return name, nil
}

func cleanupFile(l *slog.Logger, name string) {
	l.Debug("cleaning up temp file", "name", name)

	if rmErr := os.Remove(name); rmErr != nil {
		l.Error("could not clean up temp file", "error", rmErr, "name", name)
		return
	}

	l.Debug("cleaned up temp file", "name", name)
}
