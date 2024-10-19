package pirate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Validator string

const (
	ListValidator    Validator = "list"
	CommandValidator Validator = "command"
)

type Auth struct {
	Token     []string  `yaml:"token"`
	Validator Validator `yaml:"validator,omitempty"`
	Run       string    `yaml:"run,omitempty"`
}

type Logging struct {
	Dir string `yaml:"dir"`
}

type Handler struct {
	Auth     Auth   `yaml:"auth,omitempty"`
	Endpoint string `yaml:"endpoint"`
	Name     string `yaml:"name"`
	Run      string `yaml:"run"`
}

type Config struct {
	Logging  Logging   `yaml:"logging"`
	Handlers []Handler `yaml:"handlers"`
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

type Source string

const (
	LoadFromFlag   Source = "load-from-flag"
	LoadFromEnv    Source = "load-from-env"
	LoadFromCurDir Source = "load-from-cur-dir"
)

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

	return cfg, nil
}
