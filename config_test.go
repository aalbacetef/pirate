package pirate

import (
	_ "embed"
	"os"
	"strings"
	"testing"
)

//go:embed testdata/ship.yml
var testFile []byte

func TestLoad(t *testing.T) {
	tmp, err := os.CreateTemp("", "pirate-test-load-*")
	if err != nil {
		t.Fatalf("could not create temp file: %v", err)
	}
	defer os.Remove(tmp.Name())

	writeToTemp(t, tmp, testFile)

	t.Run("can correctly parse the file", func(tt *testing.T) {
		fpath := tmp.Name()

		cfg, src, err := Load(fpath)
		if err != nil {
			tt.Fatalf("could not load file: %v", err)
		}

		if src != LoadFromFlag {
			tt.Fatalf("(source) expected %s, got %s", LoadFromFlag, src)
		}

		wantCfg := Config{}
		wantCfg.Logging.Dir = "./logs"
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

		compareConfig(tt, cfg, wantCfg)
	})
}

func writeToTemp(t *testing.T, tmp *os.File, data []byte) {
	t.Helper()

	n := len(data)

	wrote, err := tmp.Write(data)
	if err != nil {
		t.Fatalf("could not write to temp file: %v", err)
	}

	if wrote != n {
		t.Fatalf("unexpected number of bytes, want %d, wrote %d", n, wrote)
	}

	if err := tmp.Sync(); err != nil {
		t.Fatalf("could not sync file: %v", err)
	}
}

func compareConfig(t *testing.T, got, want Config) {
	t.Helper()

	if got.Logging.Dir != want.Logging.Dir {
		t.Fatalf("(logging) got %s, want %s", got.Logging.Dir, want.Logging.Dir)
	}

	gotN := len(got.Handlers)
	wantN := len(want.Handlers)

	if gotN != wantN {
		t.Fatalf("(handlers) got %d handlers, want %d", gotN, wantN)
	}

	for k, handler := range got.Handlers {
		wantHandler := want.Handlers[k]

		if handler.Endpoint != wantHandler.Endpoint {
			t.Fatalf("(handlers[%d].Endpoint) got '%s', want '%s'", k, handler.Endpoint, wantHandler.Endpoint)
		}

		if handler.Name != wantHandler.Name {
			t.Fatalf("(handlers[%d].Name) got '%s', want '%s'", k, handler.Name, wantHandler.Name)
		}

		gotLines, wantLines := strings.Split(strings.TrimSpace(handler.Run), "\n"), strings.Split(wantHandler.Run, "\n")
		gotN, wantN := len(gotLines), len(wantLines)

		if gotN != wantN {
			t.Fatalf("(handlers[%d].Run) got %d entries, want %d", k, gotN, wantN)
		}

		for index, line := range gotLines {
			line = strings.TrimSpace(line)
			if line != wantLines[index] {
				t.Fatalf("(handlers[%d].Run) mismatch on line %d:\n  got  '%s'\n  want '%s'", k, index, line, wantLines[index])
			}
		}
	}
}
