package pirate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Validator is the method of validation being used, either a command or a token from a list.
type Validator string

const (
	ListValidator    Validator = "list"
	CommandValidator Validator = "command"
)

// Auth specifies the authentication of the incoming request.
// If Validator is a ListValidator, then the token of the request must match a token of the list
// If Validator is a CommandValidator, then the value of Run is executed and considered successful if exit code = 0.
type Auth struct {
	Token     []string  `yaml:"token"`
	Validator Validator `yaml:"validator"`
	Run       string    `yaml:"run"`
}

type Logging struct {
	Dir string `yaml:"dir"`
}

// Handler waits for a webhook handler to come in and runs it if authenatication passes.
type Handler struct {
	Auth     Auth   `yaml:"auth"`
	Endpoint string `yaml:"endpoint"`
	Name     string `yaml:"name"`
	Run      string `yaml:"run"`
}

type Config struct {
	Server struct {
		Host           string   `yaml:"host"`
		Port           int      `yaml:"port"`
		Logging        Logging  `yaml:"logging"`
		RequestTimeout Duration `yaml:"request-timeout"`
	} `yaml:"server"`
	Handlers []Handler `yaml:"handlers"`
}

func (cfg Config) Valid() error { //nolint:gocognit
	if cfg.Server.Port == 0 {
		return MustBeSetError{"port"}
	}

	if cfg.Server.Logging.Dir == "" {
		return MustBeSetError{"logging.dir"}
	}

	for k, handler := range cfg.Handlers {
		label := fmt.Sprintf("handler[%d]", k)
		if handler.Endpoint == "" {
			return MustBeSetError{label + ".endpoint"}
		}

		switch handler.Auth.Validator {
		default:
			return MustBeSetError{label + ".auth.validator"}
		case CommandValidator:
			if strings.TrimSpace(handler.Auth.Run) == "" {
				return MustBeSetError{label + ".auth.run"}
			}
		case ListValidator:
			if len(handler.Auth.Token) == 0 {
				return MustBeSetError{label + ".auth.tokens"}
			}
		}

		if handler.Name == "" {
			return MustBeSetError{label + ".name"}
		}

		if strings.TrimSpace(handler.Run) == "" {
			return MustBeSetError{label + ".run"}
		}
	}

	return nil
}

type MustBeSetError struct {
	field string
}

func (e MustBeSetError) Error() string {
	return fmt.Sprintf("field '%s' must be set", e.field)
}

// Load will attempt to load the config from the following
// sources (in order):
//   - flag value (if passed)
//   - Env Var ($PIRATE_CONFIG_PATH)
//   - current working directory
//
// It will return the source used, which aids in debugging.
func Load(fpath string) (Config, Source, error) {
	source := determineSource(fpath)

	switch source {
	case LoadFromFlag:
		cfg, err := loadConfigFromFile(fpath)
		return cfg, source, err

	case LoadFromEnv:
		cfg, err := loadConfigFromFile(os.Getenv(ConfigEnvVar))
		return cfg, source, err

	case LoadFromCurDir:
		wd, err := os.Getwd()
		if err != nil {
			return Config{}, source, fmt.Errorf("os.Getwd failed: %w", err)
		}

		cfg, err := loadConfigFromFile(filepath.Join(wd, defaultFilename))

		return cfg, source, err

	default:
		return Config{}, "", errors.New("no source could be determined")
	}
}

// Source is where the config was read from.
type Source string

const (
	LoadFromFlag   Source = "load-from-flag"
	LoadFromEnv    Source = "load-from-env"
	LoadFromCurDir Source = "load-from-cur-dir"
)

// default variables.
const (
	ConfigEnvVar    = "PIRATE_CONFIG_PATH"
	defaultFilename = "ship.yml"
)

func determineSource(fpath string) Source {
	if fpath != "" {
		return LoadFromFlag
	}

	if os.Getenv(ConfigEnvVar) != "" {
		return LoadFromEnv
	}

	return LoadFromCurDir
}

type FileNotFoundError struct {
	Path string
}

func (e FileNotFoundError) Error() string {
	return fmt.Sprintf("file not found: '%s'", e.Path)
}

func loadConfigFromFile(fpath string) (Config, error) {
	cfg := Config{}

	absPath, err := filepath.Abs(fpath)
	if err != nil {
		return cfg, fmt.Errorf("filepath.Abspath failed when loading config from file: %w", err)
	}

	data, err := os.ReadFile(fpath)
	if err != nil {
		return cfg, fmt.Errorf("could not read file '%s': %w", absPath, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("could not unmarhsal config: %w", err)
	}

	// set default values if any
	if cfg.Server.Host == "" {
		cfg.Server.Host = defaultHost
	}

	if err := cfg.Valid(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

const (
	defaultHost           = "localhost"
	defaultRequestTimeout = 5 * time.Minute
)

type Duration struct {
	time.Duration
}

func (d *Duration) MarshalJSON() ([]byte, error) {
	dur := d.Duration

	if dur == 0 {
		dur = defaultRequestTimeout
	}

	return []byte(dur.String()), nil
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	str := string(data)

	if str == "" {
		d.Duration = defaultRequestTimeout
		return nil
	}

	dur, err := time.ParseDuration(str)
	if err != nil {
		return fmt.Errorf("could not parse '%s': %w", str, err)
	}

	d.Duration = dur

	return nil
}
