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

	"github.com/aalbacetef/pirate/scheduler"
)

type Server struct {
	// Should be READ only after initialization.
	cfg Config

	logger            *slog.Logger
	validationTimeout time.Duration
	cleanup           []func()
	schedulers        []Scheduler
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

type Scheduler interface {
	Start() error
	Pause() error
	Name() string
	Add(*scheduler.Job) error
}

// @TODO: handle log to Stdout.
func NewServer(cfg Config) (*Server, error) {
	fd, cleanupFn, err := initializeLogging(cfg.Server.Logging.Dir)
	if err != nil {
		return nil, err
	}

	cleanup := make([]func(), 0, 1+len(cfg.Handlers))
	cleanup = append(cleanup, cleanupFn)

	srv := &Server{
		cfg: cfg,
		logger: slog.New(slog.NewJSONHandler(
			fd,
			&slog.HandlerOptions{Level: slog.LevelDebug.Level()},
		)),
		validationTimeout: defaultValidationTimeout,
	}

	schedulers := make([]Scheduler, 0, len(cfg.Handlers))
	for k, handler := range cfg.Handlers {
		name := handler.Name

		var (
			schedErr error
			sched    Scheduler
		)

		switch handler.Policy {
		case Queue:
			sched, schedErr = scheduler.NewPipeline(handler.Name)
			if schedErr != nil {
				return nil, fmt.Errorf(
					"failed to create scheduler(name=%s): %w",
					name, schedErr,
				)
			}
		case Parallel:
			sched, schedErr = scheduler.NewParallel(handler.Name)
			if schedErr != nil {
				return nil, fmt.Errorf(
					"failed to create scheduler(name=%s): %w",
					name, schedErr,
				)
			}
		case Drop:
			sched, schedErr = scheduler.NewDrop(handler.Name)
			if schedErr != nil {
				return nil, fmt.Errorf(
					"failed to create scheduler(name=%s): %w",
					name, schedErr,
				)
			}
		default:
			return nil, fmt.Errorf("unknown policy: '%s'", handler.Policy)
		}

		schedulers = append(schedulers, sched)
		if err := schedulers[k].Start(); err != nil {
			return nil, fmt.Errorf(
				"failed to start scheulder(name=%s): %w",
				name, err,
			)
		}

		cleanup = append(cleanup, func() {
			if err := schedulers[k].Pause(); err != nil {
				srv.logger.Error(
					"could not pause scheduler",
					"name", name,
					"error", err,
				)
			}
		})
	}

	srv.cleanup = cleanup
	srv.schedulers = schedulers

	return srv, nil
}

const (
	LogToStdOut        = ":stdout:"
	LogTimestampFormat = "2006-01-02--15-04-05"
)

func initializeLogging(loggingDir string) (*os.File, func(), error) {
	loggingDir = strings.TrimSpace(loggingDir)
	noop := func() {}

	if loggingDir == LogToStdOut {
		return os.Stdout, noop, nil
	}

	if strings.HasPrefix(loggingDir, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, nil, fmt.Errorf("could not get user home dir: %w", err)
		}

		loggingDir = filepath.Join(
			homeDir,
			strings.Replace(loggingDir, "~/", "", 1),
		)
	}

	// @TODO: add timestamp to filename
	loggingDir, err := filepath.Abs(loggingDir)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"could not make absolute filepath: %w",
			err,
		)
	}

	timestamp := (time.Now()).Format(LogTimestampFormat)
	fpath := filepath.Join(loggingDir, fmt.Sprintf("%s.log", timestamp))

	// make directory if doesn't exist
	if mkErr := os.MkdirAll(loggingDir, dirPerms); mkErr != nil {
		return nil, nil, fmt.Errorf(
			"could not create logging directory (%s): %w",
			loggingDir, mkErr,
		)
	}

	fd, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePerms)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"could not create log file (%s): %w",
			fpath, err,
		)
	}

	return fd, func() { fd.Close() }, nil
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

		// no reason to let strangers know an error occurred.
		w.WriteHeader(http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), srv.validationTimeout)
	defer cancel()

	if validationErr := validateRequest(ctx, srv.logger, handler.Name, handler.Auth, req); validationErr != nil {
		// no reason to let strangers know the endpoint is valid.
		w.WriteHeader(http.StatusNotFound)

		if errors.Is(validationErr, ErrAuthFailed) {
			logger.Debug("authentication failed")
			return
		}

		logger.Error("unexpected request validation error", "error", validationErr)

		return
	}

	// kick off task and return.
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		srv.logger.Error("error reading the request body", "error", err)
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	req.Body.Close()

	headers := make(map[string]string, len(req.Header))
	for key := range req.Header {
		headers[key] = req.Header.Get(key)
	}

	// we don't pass the context as Do should run in the background independent of the request.
	go srv.Do(&handler, headers, payload)

	w.WriteHeader(http.StatusOK)
}

var (
	ErrAuthFailed       = errors.New("authentication failed")
	ErrUnknownValidator = errors.New("unknown validator")
)

const TokenHeaderField = "X-Authorization"

func validateRequest(ctx context.Context, logger *slog.Logger, name string, authCfg Auth, req *http.Request) error {
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
		if err := runScript(
			ctx,
			"pirate-command-*",
			authCfg.Run,
			[]string{
				fmt.Sprintf("PIRATE_TOKEN='%s'", token),
				fmt.Sprintf("PIRATE_NAME='%s'", name),
			},
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
func (srv *Server) Do(handler *Handler, headers map[string]string, payload []byte) {
	l := srv.logger.With(
		"Fn", "srv.Do",
		"handler", handler.Name,
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

	index := -1
	for k, h := range srv.cfg.Handlers {
		if h.Name == handler.Name {
			index = k
			break
		}
	}

	if index == -1 {
		l.Error("could not find matching scheduler", "handler.Name", handler.Name)
		return
	}

	sched := srv.schedulers[index]

	job, err := scheduler.NewJob(func(runCtx context.Context) error {
		ctx, cancel := context.WithTimeout(runCtx, DoTimeout)
		defer cancel()

		if err := runScript(ctx, "pirate-webhook-script-*", handler.Run, env, l); err != nil {
			l.Error("error running script", "error", err)
		}

		return nil
	})

	if err != nil {
		l.Error("could not create new job", "error", err)
		return
	}

	if err := sched.Add(job); err != nil {
		l.Error("could not add job to scheduler", "error", err)
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
