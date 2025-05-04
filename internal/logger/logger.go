package logger

import (
	"io"
	"os"
	"path/filepath"

	"double-take-go-reborn/internal/config"

	log "github.com/sirupsen/logrus"
)

// Init initializes the global logger based on the provided configuration.
func Init(cfg config.LogConfig) error {
	// Set log level
	level, err := log.ParseLevel(cfg.Level)
	if err != nil {
		log.Warnf("Invalid log level '%s', defaulting to 'info': %v", cfg.Level, err)
		level = log.InfoLevel
	}
	log.SetLevel(level)

	// Set log format (e.g., JSON or Text)
	// Using TextFormatter for now, similar to default log, but structured
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	// Set output: file and/or stdout
	var writers []io.Writer
	writers = append(writers, os.Stdout) // Always log to stdout for container logs

	if cfg.File != "" {
		// Ensure the directory for the log file exists
		logDir := filepath.Dir(cfg.File)
		if err := os.MkdirAll(logDir, 0750); err != nil {
			log.Errorf("Failed to create log directory '%s': %v", logDir, err)
			// Continue without file logging if directory creation fails
		} else {
			file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
			if err != nil {
				log.Errorf("Failed to open log file '%s': %v", cfg.File, err)
				// Continue without file logging if file opening fails
			} else {
				writers = append(writers, file)
				// Consider adding a mechanism to close the file on shutdown
				log.Infof("Logging additionally to file: %s", cfg.File)
			}
		}
	}

	if len(writers) > 0 {
		log.SetOutput(io.MultiWriter(writers...))
	} else {
		// Fallback to stdout if no writers configured (shouldn't happen with current logic)
		log.SetOutput(os.Stdout)
	}

	log.Info("Logger initialized")
	return nil
}
