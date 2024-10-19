package pirate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	DoTimeout    = 5 * time.Minute
	tickInterval = 10 * time.Second
)

// Do runs after a request has been validated.
// @TODO: maybe enforce Content-Type: application/json ?
// @TODO: add optional shell setting to config.
// @TODO: add handler timeout setting.
func (srv *Server) Do(h Handler, headers map[string]string, payload []byte) {
	l := srv.logger.With(
		"Fn", "srv.Do",
		"handler", h.Name,
	)

	l.Info("starting handler")

	buf := &bytes.Buffer{}

	if err := json.NewEncoder(buf).Encode(headers); err != nil {
		l.Error("could not encode headers", "error", err)
		return
	}

	env := []string{
		fmt.Sprintf("PIRATE_HEADERS=%s", buf.String()),
		fmt.Sprintf("PIRATE_BODY='%s'", string(payload)),
	}

	file, err := os.CreateTemp("", "pirate-webhook-script-*")
	if err != nil {
		l.Error("could not create temp file", "error", err)
		return
	}

	name := file.Name()

	defer func() {
		l.Info("cleaning up temp file", "name", name)

		if rmErr := os.Remove(name); rmErr != nil {
			l.Error("could not clean up temp file", "error", rmErr, "filepath", name)
			return
		}

		l.Info("done")
	}()

	script := h.Run

	scriptLen := len(script)

	n, err := file.WriteString(script)
	if err != nil {
		l.Error("could not write run lines to script", "error", err)
		return
	}

	if n != scriptLen {
		l.Error("wrote insufficient number of bytes", "n", n, "want", scriptLen)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), DoTimeout)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"bash", name,
	)
	cmd.Env = env

	stderr := newSafeBuffer()
	stdout := newSafeBuffer()

	cmd.Stderr = stderr
	cmd.Stdout = stdout

	l.Info("commencing execution")

	if err := cmd.Start(); err != nil {
		l.Error("could not start command", "error", err)
		return
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
	}

	flush(stdout, stderr, l)

	l.Info("execution finished", "code", code)
}

func flush(stdout, stderr *safeBuffer, l *slog.Logger) {
	outStr := stdout.String()
	errStr := stderr.String()

	stdout.Reset()
	stderr.Reset()

	if len(strings.TrimSpace(outStr)) > 0 {
		l.Info(outStr)
	}

	if len(strings.TrimSpace(errStr)) > 0 {
		l.Error(errStr)
	}
}
