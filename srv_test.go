package pirate

import (
	"bytes"
	_ "embed"
	"testing"
)

//go:embed testdata/ship.stdout.yml
var testConfigFile []byte

func TestServerInit(t *testing.T) {
	cfg, err := loadConfig(bytes.NewReader(testConfigFile))
	if err != nil {
		t.Fatalf("could not load config file: %v", err)
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("could not initialize server: %v", err)
	}

	t.Run("logger should be set", func(tt *testing.T) {
		if server.logger == nil {
			tt.Fatalf("logger is nil")
		}
	})

	t.Run("validation timeout is set", func(tt *testing.T) {
		if server.validationTimeout != defaultValidationTimeout {
			tt.Fatalf("validation timeout was not set")
		}
	})

	t.Run("cleanup was set", func(tt *testing.T) {
		wantN := 1 // there should be the single logging cleanup function
		gotN := len(server.cleanup)

		if gotN != wantN {
			tt.Fatalf("got %d, want %d", gotN, wantN)
		}
	})
}

func TestServerCleanup(t *testing.T) {
	t.Run("close should call all cleanup handlers", func(tt *testing.T) {
		wasCalled := []bool{false, false, false}
		srv := &Server{}

		for k := range len(wasCalled) {
			srv.cleanup = append(srv.cleanup, func() {
				wasCalled[k] = true
			})
		}

		srv.Close()

		tt.Run("all handlers were called", func(ttt *testing.T) {
			for k, v := range wasCalled {
				if !v {
					ttt.Fatalf("%d: not called", k)
				}
			}
		})

		tt.Run("should have set cleanup to nil", func(ttt *testing.T) {
			if srv.cleanup != nil {
				ttt.Fatalf("did not set srv.cleanup to nil")
			}
		})
	})
}
