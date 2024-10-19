package pirate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type Server struct {
	// Should be READ only after initialization.
	cfg Config

	logger *slog.Logger

	validationTimeout time.Duration
}

const (
	defaultValidationTimeout = 5 * time.Second
)

func NewServer(cfg Config) (*Server, error) {
	srv := &Server{
		cfg: cfg,
		logger: slog.New(slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{Level: slog.LevelDebug.Level()},
		)),
		validationTimeout: defaultValidationTimeout,
	}

	return srv, nil
}

var HandlerNotFoundErr = errors.New("no matching handler was found")

func (srv *Server) FindHandler(endpoint string) (Handler, error) {
	for _, h := range srv.cfg.Handlers {
		if h.Endpoint == endpoint {
			return h, nil
		}
	}

	return Handler{}, HandlerNotFoundErr
}

// HandleRequest is the main entrypoint of the server. It will first check if the
// request is a valid endpoint and passes auth checks. Then it will spin off a
// goroutine that executes the actual task.
// @TODO: queue multiple executions of the same endpoint.
func (srv *Server) HandleRequest(w http.ResponseWriter, req *http.Request) {
	logger := srv.logger.With("Fn", "Server.HandleRequest", "req.URL.Path", req.URL.Path)

	logger.Debug("checking matching handler...")

	handler, err := srv.FindHandler(req.URL.Path)
	if errors.Is(err, HandlerNotFoundErr) {
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

		if !errors.Is(err, ErrAuthFailed) {
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

var ErrAuthFailed = errors.New("authentication failed")

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

		cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`"%s"`, authCfg.Run))
		cmd.Env = append(cmd.Env, "PIRATE_TOKEN="+token)

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command returned error: %w", err)
		}

		return nil

	default:
		return ErrUnknownValidator
	}
}

var ErrUnknownValidator = errors.New("unknown validator")

const TokenHeaderField = "X-Authorization"
