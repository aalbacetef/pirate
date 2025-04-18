package pirate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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

// Logging defines the directory where logs should be written.
type Logging struct {
	Dir string `yaml:"dir"`
}

// Handler waits for a webhook handler to come in and runs it if authenatication passes.
type Handler struct {
	Auth     Auth            `yaml:"auth"`
	Endpoint string          `yaml:"endpoint"`
	Name     string          `yaml:"name"`
	Run      string          `yaml:"run"`
	Policy   ExecutionPolicy `yaml:"policy,omitempty"`
}

type ExecutionPolicy string

const (
	Drop     ExecutionPolicy = "drop"
	Parallel ExecutionPolicy = "parallel"
	Queue    ExecutionPolicy = "queue"
)

// Config defines the configuration for the pirate server and its handlers.
type Config struct {
	Server struct {
		Host           string   `yaml:"host"`
		Port           int      `yaml:"port"`
		Logging        Logging  `yaml:"logging"`
		RequestTimeout Duration `yaml:"request-timeout"`
		MaxHeaderBytes ByteSize `yaml:"max-header-bytes"`
	} `yaml:"server"`
	Handlers []Handler `yaml:"handlers"`
}

// Valid will fail if fields are missing.
// Note that it expects optional fields to be set before being called.
func (cfg Config) Valid() error { //nolint:gocognit
	if cfg.Server.Host == "" {
		return MustBeSetError{"host"}
	}

	if cfg.Server.Port == 0 {
		return MustBeSetError{"port"}
	}

	if cfg.Server.Logging.Dir == "" {
		return MustBeSetError{"logging.dir"}
	}
	if cfg.Server.MaxHeaderBytes.Value <= 0 {
		return MustBeSetError{"server.max-header-bytes"}
	}

	for k, handler := range cfg.Handlers {
		label := fmt.Sprintf("handler[%d]", k)
		if handler.Endpoint == "" {
			return MustBeSetError{label + ".endpoint"}
		}

		switch handler.Policy {
		default:
			return MustBeSetError{label + ".policy"}
		case Queue, Parallel, Drop:
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

// MustBeSetError represents an error indicating a required field is missing.
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

// determineSource determines the source of the configuration file.
func determineSource(fpath string) Source {
	if fpath != "" {
		return LoadFromFlag
	}

	if os.Getenv(ConfigEnvVar) != "" {
		return LoadFromEnv
	}

	return LoadFromCurDir
}

// FileNotFoundError represents an error when a config file is not found.
type FileNotFoundError struct {
	Path string
}

func (e FileNotFoundError) Error() string {
	return fmt.Sprintf("file not found: '%s'", e.Path)
}

// loadConfigFromFile loads the configuration from a specified file path.
func loadConfigFromFile(fpath string) (Config, error) {
	absPath, err := filepath.Abs(fpath)
	if err != nil {
		return Config{}, fmt.Errorf("filepath.Abspath failed when loading config from file: %w", err)
	}

	fd, err := os.Open(fpath)
	if err != nil {
		return Config{}, fmt.Errorf("could not read file '%s': %w", absPath, err)
	}

	defer fd.Close()

	return loadConfig(fd)
}

// loadConfig loads the configuration from an io.Reader.
func loadConfig(r io.Reader) (Config, error) {
	cfg := Config{}

	if err := yaml.NewDecoder(r).Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("could not unmarhsal config: %w", err)
	}

	if cfg.Server.RequestTimeout.Duration == 0 {
		cfg.Server.RequestTimeout.Duration = defaultRequestTimeout
	}

	if cfg.Server.MaxHeaderBytes.Value == 0 {
		cfg.Server.MaxHeaderBytes.Value = defaultMaxHeaderBytes // Default to 1k
	}

	// set default values if any
	if cfg.Server.Host == "" {
		cfg.Server.Host = defaultHost
	}

	for k, handler := range cfg.Handlers {
		if handler.Policy == "" {
			cfg.Handlers[k].Policy = defaultHandlerPolicy
		}
	}

	if err := cfg.Valid(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// Duration is a wrapper around time.Duration that supports JSON and YAML marshaling/unmarshaling.
type Duration struct {
	time.Duration
}

// MarshalJSON marshals the Duration to JSON.
func (d *Duration) MarshalJSON() ([]byte, error) {
	dur := d.Duration

	if dur == 0 {
		dur = defaultRequestTimeout
	}

	return []byte(dur.String()), nil
}

// UnmarshalJSON unmarshals the Duration from JSON.
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

// MarshalYAML marshals the Duration to YAML.
func (d *Duration) MarshalYAML() ([]byte, error) {
	return d.MarshalJSON()
}

// UnmarshalYAML unmarshals the Duration from YAML.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	return d.UnmarshalJSON([]byte(node.Value))
}

const (
	Kilobyte = 1024
	Megabyte = 1024 * Kilobyte
	Gigabyte = 1024 * Megabyte
)

type ByteSize struct {
	Value int
}

// MarshalJSON marshals the Duration to JSON.
func (b *ByteSize) MarshalJSON() ([]byte, error) {
	bs := b.Value
	if bs == 0 {
		bs = 1024 // Default to 1k
	}

	return []byte(strconv.Itoa(bs)), nil
}

// UnmarshalJSON unmarshals the Duration from JSON.
func (b *ByteSize) UnmarshalJSON(data []byte) error {
	str := strings.TrimSpace(string(data))

	if str == "" {
		b.Value = Kilobyte // Default to 1k
		return nil
	}

	multipliers := map[string]int{
		"k": Kilobyte,
		"M": Megabyte,
		"G": Gigabyte,
	}

	multiplier := 1
	numericPart := ""

	switch {
	case strings.HasSuffix(str, "k"):
		multiplier = multipliers["k"]
		numericPart = strings.TrimSuffix(str, "k")
	case strings.HasSuffix(str, "M"):
		multiplier = multipliers["M"]
		numericPart = strings.TrimSuffix(str, "M")
	case strings.HasSuffix(str, "G"):
		multiplier = multipliers["G"]
		numericPart = strings.TrimSuffix(str, "G")
	default:
		numericPart = str
	}

	num, err := strconv.Atoi(numericPart)
	if err != nil {
		return fmt.Errorf("invalid numeric value: %w", err)
	}

	result := num * multiplier
	if result < 0 {
		return errors.New("value overflowed maximum int size")
	}
	b.Value = result

	return nil
}

func (b *ByteSize) UnmarshalYAML(node *yaml.Node) error {
	return b.UnmarshalJSON([]byte(node.Value))
}
