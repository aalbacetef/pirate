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

func runScript(ctx context.Context, fname, contents string, env []string, l *slog.Logger) error {
	fd, err := os.CreateTemp("", fname)
	if err != nil {
		return fmt.Errorf("could not create script: %w", err)
	}

	name := fd.Name()

	defer func() {
		l.Debug("cleaning up temp file", "name", name)

		if rmErr := os.Remove(name); rmErr != nil {
			l.Error("could not clean up temp file", "error", rmErr, "name", name)
			return
		}

		l.Debug("cleaned up temp file", "name", name)
	}()

	wroteBytes, err := fd.WriteString(contents)
	if err != nil {
		return fmt.Errorf("error writing script: %w", err)
	}

	wantBytes := len([]byte(contents))

	if wantBytes != wroteBytes {
		return fmt.Errorf("expected %d bytes, wrote %d", wantBytes, wroteBytes)
	}

	fd.Close()

	cmd := exec.CommandContext(ctx, "bash", name)
	cmd.Env = env

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
			case <-ctx.Done():
				return
			case <-ticker.C:
				flush(stdout, stderr, l)
			}
		}
	}()

	code := 0
	if err := cmd.Wait(); err != nil {
		code = 1

		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		}

		return fmt.Errorf("failed (exit code=%d): %w", code, err)
	}

	flush(stdout, stderr, l)

	return nil
}
