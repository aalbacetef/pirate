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
		wantCfg.Logging.Dir = "~/logs"
		wantCfg.Handlers = []Handler{
			{
				Endpoint: "/webhooks/repo-a",
				Name:     "handle A repo's update",
				Run: `echo "body:" "$PIRATE_BODY"` +
					`echo "headers:" "$(echo "$PIRATE_HEADERS" | jq )"` +
					`echo "header param:" "$PIRATE_HEADERS_SOME_PARAM"`,
			},
			{
				Endpoint: "/another-path",
				Name:     "new release",
				Run: `echo "body:" "$PIRATE_BODY"` +
					`./some-script.sh $($PIRATE_BODY | jq -r '.token')`,
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

	for k, h := range got.Handlers {
		w := want.Handlers[k]

		if h.Endpoint != w.Endpoint {
			t.Fatalf("(handlers[%d].Endpoint) got '%s', want '%s'", k, h.Endpoint, w.Endpoint)
		}

		if h.Name != w.Name {
			t.Fatalf("(handlers[%d].Name) got '%s', want '%s'", k, h.Name, w.Name)
		}

		gotN, wantN := len(h.Run), len(w.Run)

		if gotN != wantN {
			t.Fatalf("(handlers[%d].Run) got %d entries, want %d", k, gotN, wantN)
		}

		want := strings.TrimSpace(w.Run)
		got := strings.TrimSpace(h.Run)
		if got != want {
			t.Fatalf(
				"(handlers[%d].Run) mismatch: got '%s', want '%s'",
				k, got, want,
			)
		}
	}
}
