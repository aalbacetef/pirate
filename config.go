package pirate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	Validator Validator `yaml:"validator,omitempty"`
	Run       string    `yaml:"run,omitempty"`
}

type Logging struct {
	Dir string `yaml:"dir"`
}

// Handler waits for a webhook handler to come in and runs it if authenatication passes.
type Handler struct {
	Auth     Auth   `yaml:"auth,omitempty"`
	Endpoint string `yaml:"endpoint"`
	Name     string `yaml:"name"`
	Run      string `yaml:"run"`
}

type Config struct {
	Server struct {
		Port    int     `yaml:"port"`
		Logging Logging `yaml:"logging"`
	} `yaml:"server"`
	Handlers []Handler `yaml:"handlers"`
}

func (cfg Config) Valid() error {
	if cfg.Server.Port == 0 {
		return errors.New("port must be set")
	}

	return nil
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

	if err := cfg.Valid(); err != nil {
		return cfg, err
	}

	return cfg, nil
}
