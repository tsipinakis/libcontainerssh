package config

import (
	"github.com/containerssh/containerssh/config"
)

// ConfigSaver is a utility to store configuration
type ConfigSaver interface {
	// Store stores the passed configuration and returns an error on failure.
	Save(config *config.AppConfig) error
}
