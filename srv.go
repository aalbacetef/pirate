package pirate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Server struct {
	// Should be READ only after initialization.
	cfg Config

	logger *slog.Logger

	validationTimeout time.Duration

	cleanup []func()
}

func (srv *Server) Close() {
	n := len(srv.cleanup)
	for k := range n {
		fn := srv.cleanup[n-k-1]
		fn()
	}

	srv.cleanup = nil
}

const (
	defaultValidationTimeout = 5 * time.Second
	dirPerms                 = 0o744
	filePerms                = 0o644
)

// @TODO: the code for setting up logging file / dir should also handle the case
// where we want to log to Stdout
func NewServer(cfg Config) (*Server, error) {
	loggingDir := strings.TrimSpace(cfg.Server.Logging.Dir)
	if strings.HasPrefix(loggingDir, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not get user home dir: %w", err)
		}

		loggingDir = filepath.Join(
			homeDir,
			strings.Replace(loggingDir, "~/", "", 1),
		)
	}

	// @TODO: add timestamp to filename
	loggingDir, err := filepath.Abs(loggingDir)
	if err != nil {
		return nil, fmt.Errorf(
			"could not make absolute filepath: %w",
			err,
		)
	}

	fpath := filepath.Join(loggingDir, "app.log")

	// make directory if doesn't exist
	if mkErr := os.MkdirAll(loggingDir, dirPerms); mkErr != nil {
		return nil, fmt.Errorf(
			"could not create logging directory (%s): %w",
			cfg.Server.Logging.Dir, mkErr,
		)
	}

	fd, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePerms)
	if err != nil {
		return nil, fmt.Errorf(
			"could not create log file (%s): %w",
			fpath, err,
		)
	}

	closeFn := func() {
		fd.Close()
	}

	srv := &Server{
		cfg: cfg,
		logger: slog.New(slog.NewJSONHandler(
			fd,
			&slog.HandlerOptions{Level: slog.LevelDebug.Level()},
		)),
		validationTimeout: defaultValidationTimeout,
	}

	srv.cleanup = append(srv.cleanup, closeFn)

	return srv, nil
}

var ErrHandlerNotFound = errors.New("no matching handler was found")

func (srv *Server) FindHandler(endpoint string) (Handler, error) {
	for _, h := range srv.cfg.Handlers {
		if h.Endpoint == endpoint {
			return h, nil
		}
	}

	return Handler{}, ErrHandlerNotFound
}

// HandleRequest is the main entrypoint of the server. It will first check if the
// request is a valid endpoint and passes auth checks. Then it will spin off a
// goroutine that executes the actual task.
// @TODO: queue multiple executions of the same endpoint.
func (srv *Server) HandleRequest(w http.ResponseWriter, req *http.Request) {
	logger := srv.logger.With("Fn", "Server.HandleRequest", "req.URL.Path", req.URL.Path)

	logger.Debug("checking matching handler...")

	handler, err := srv.FindHandler(req.URL.Path)
	if errors.Is(err, ErrHandlerNotFound) {
		logger.Debug("no matching handler, returning 404")

		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err != nil {
		logger.Error("srv.FindHandler returned an unexpected error", "error", err)

		// no reason to let strangers now an error occurred.
		w.WriteHeader(http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), srv.validationTimeout)
	defer cancel()

	if err := validateRequest(ctx, srv.logger, handler.Auth, req); err != nil {

		// no reason to let strangers now the endpoint is valid.
		w.WriteHeader(http.StatusNotFound)

		if errors.Is(err, ErrAuthFailed) {
			logger.Debug("authentication failed")
			return
		}

		logger.Error("unexpected request validation error", "error", err)

		return
	}

	// kick off task and return.
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		srv.logger.Error("error reading the request body", "error", err)
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	headers := make(map[string]string, len(req.Header))
	for key := range req.Header {
		headers[key] = req.Header.Get(key)
	}

	go srv.Do(handler, headers, payload)

	w.WriteHeader(http.StatusOK)
}

var (
	ErrAuthFailed       = errors.New("authentication failed")
	ErrUnknownValidator = errors.New("unknown validator")
)

const TokenHeaderField = "X-Authorization"

func validateRequest(ctx context.Context, logger *slog.Logger, authCfg Auth, req *http.Request) error {
	token := req.Header.Get(TokenHeaderField)

	switch authCfg.Validator {
	case ListValidator:
		logger.Debug("using list validator")

		for _, tk := range authCfg.Token {
			if token == tk {
				return nil
			}
		}

		return ErrAuthFailed

	case CommandValidator:
		logger.Debug("using command validator")
		logger.Debug("run is: ", "run", authCfg.Run)

		if err := runScript(
			ctx,
			"pirate-command-*",
			authCfg.Run,
			[]string{"PIRATE_TOKEN=" + token},
			logger,
		); err != nil {
			return fmt.Errorf("command returned error: %w", err)
		}

		return nil

	default:
		return ErrUnknownValidator
	}
}

const DoTimeout = 5 * time.Minute

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

	ctx, cancel := context.WithTimeout(context.Background(), DoTimeout)
	defer cancel()

	if err := runScript(ctx, "pirate-webhook-script-*", h.Run, env, l); err != nil {
		l.Error("error running script", "error", err)
	}
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
