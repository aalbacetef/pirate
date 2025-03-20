package pirate

import (
	"bytes"
	_ "embed"
	"strings"
	"testing"
	"time"
)

//go:embed testdata/ship.yml
var testFilePopulated []byte

//go:embed testdata/ship.only-required.yml
var testFileOnlyRequired []byte

func TestLoad(t *testing.T) {
	cfg, err := loadConfig(bytes.NewReader(testFilePopulated))
	if err != nil {
		t.Fatalf("could not load file: %v", err)
	}

	wantCfg := Config{}
	wantCfg.Server.RequestTimeout = Duration{150 * time.Second}
	wantCfg.Server.Port = 3939
	wantCfg.Server.Logging.Dir = "./logs"
	wantCfg.Handlers = []Handler{
		{
			Endpoint: "/webhooks/simple",
			Name:     "simple webhook handler",
			Run: `` +
				`SOME_VAR="some-variable"` + "\n" +
				`echo "SOME_VAR: $SOME_VAR"` + "\n" +
				`echo "body: $PIRATE_BODY"` + "\n" +
				`echo "headers: $PIRATE_HEADERS"` + "\n" +
				`echo "header param: $PIRATE_HEADERS_SOME_PARAM"` + "",
		},
		{
			Endpoint: "/new-release",
			Name:     "new release",
			Run: `` +
				`echo "this should never run!"` + "\n" +
				`./some-script.sh $("$PIRATE_BODY" | jq -r '.token')`,
		},
	}

	compareConfig(t, cfg, wantCfg)
}

func TestLoadsDefaults(t *testing.T) {
	rdr := bytes.NewReader(testFileOnlyRequired)

	cfg, err := loadConfig(rdr)
	if err != nil {
		t.Fatalf("error parsing file: %v", err)
	}

	t.Run("default host was set", func(tt *testing.T) {
		got := cfg.Server.Host
		want := defaultHost

		if got != want {
			tt.Fatalf("got '%s', want '%s'", got, want)
		}
	})

	t.Run("default request timeout", func(tt *testing.T) {
		got := cfg.Server.RequestTimeout.String()
		want := cfg.Server.RequestTimeout.String()

		if got != want {
			tt.Fatalf("got '%s', want '%s'", got, want)
		}
	})
}

func TestConfigIsValid(t *testing.T) {
	baseCfg, err := loadConfig(bytes.NewReader(testFileOnlyRequired))
	if err != nil {
		t.Fatalf("could not load base file: %v", err)
	}

	t.Run("should validate port", func(tt *testing.T) {
		cfg := clone(baseCfg)
		cfg.Server.Port = 0

		if cfg.Valid() == nil {
			tt.Fatalf("error: should've failed")
		}
	})

	t.Run("should validate host", func(tt *testing.T) {
		cfg := clone(baseCfg)
		cfg.Server.Host = ""

		if cfg.Valid() == nil {
			tt.Fatalf("error: should've failed")
		}
	})

	t.Run("should validate auth.handler.validator", func(tt *testing.T) {
		cfg := clone(baseCfg)
		cfg.Handlers[0].Auth.Validator = ""

		if cfg.Valid() == nil {
			tt.Fatalf("error: should've failed")
		}
	})

	t.Run("should validate auth.handler.tokens when validator is list", func(tt *testing.T) {
		tt.Run("fail if nil", func(ttt *testing.T) {
			cfg := clone(baseCfg)
			cfg.Handlers[0].Auth.Validator = ListValidator
			cfg.Handlers[0].Auth.Token = nil

			if cfg.Valid() == nil {
				ttt.Fatalf("should've failed")
			}
		})

		tt.Run("fail if empty", func(ttt *testing.T) {
			cfg := clone(baseCfg)
			cfg.Handlers[0].Auth.Validator = ListValidator
			cfg.Handlers[0].Auth.Token = []string{}

			if cfg.Valid() == nil {
				ttt.Fatalf("should've failed")
			}
		})
	})
}

func clone[T any](v T) T { //nolint:ireturn
	ptr := &v
	return *ptr
}

func compareConfig(t *testing.T, got, want Config) {
	t.Helper()

	gotTime := got.Server.RequestTimeout.String()
	wantTime := want.Server.RequestTimeout.String()

	if gotTime != wantTime {
		t.Fatalf(
			"(request-time) got %s, want %s",
			gotTime, wantTime,
		)
	}

	if got.Server.Port != want.Server.Port {
		t.Fatalf(
			"(port) got %d, want %d",
			got.Server.Port, want.Server.Port,
		)
	}

	if got.Server.Logging.Dir != want.Server.Logging.Dir {
		t.Fatalf(
			"(logging) got %s, want %s",
			got.Server.Logging.Dir, want.Server.Logging.Dir,
		)
	}

	gotN := len(got.Handlers)
	wantN := len(want.Handlers)

	if gotN != wantN {
		t.Fatalf("(handlers) got %d handlers, want %d", gotN, wantN)
	}

	for k, handler := range got.Handlers {
		testCompareHandler(t, k, &handler, &want.Handlers[k])
	}
}

func testCompareHandler(t *testing.T, k int, handler, wantHandler *Handler) {
	t.Helper()

	if handler.Endpoint != wantHandler.Endpoint {
		t.Fatalf("(handlers[%d].Endpoint) got '%s', want '%s'", k, handler.Endpoint, wantHandler.Endpoint)
	}

	if handler.Name != wantHandler.Name {
		t.Fatalf("(handlers[%d].Name) got '%s', want '%s'", k, handler.Name, wantHandler.Name)
	}

	gotLines := strings.Split(strings.TrimSpace(handler.Run), "\n")
	wantLines := strings.Split(strings.TrimSpace(wantHandler.Run), "\n")
	gotN, wantN := len(gotLines), len(wantLines)

	if gotN != wantN {
		t.Fatalf("(handlers[%d].Run) got %d entries, want %d", k, gotN, wantN)
	}

	for index, line := range gotLines {
		line = strings.TrimSpace(line)
		if line != strings.TrimSpace(wantLines[index]) {
			t.Fatalf(
				"(handlers[%d].Run) mismatch on line %d:\n  got  '%s'\n  want '%s'",
				k, index, line, wantLines[index],
			)
		}
	}
}
