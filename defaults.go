package pirate

import "time"

const (
	// Environment variable to read the pirate config path from.
	ConfigEnvVar = "PIRATE_CONFIG_PATH"

	// Default config file name assumed.
	defaultFilename = "ship.yml"

	// Default host to serve from.
	defaultHost = "localhost"

	// Default request timeout.
	defaultRequestTimeout = 5 * time.Minute
)
