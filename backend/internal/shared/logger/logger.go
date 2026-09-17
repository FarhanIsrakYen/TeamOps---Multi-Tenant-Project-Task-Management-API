package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// New returns a structured JSON logger for every environment. Human-readable
// rendering belongs at the log collector or local CLI boundary, not in the API.
func New(environment string) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	return zerolog.New(os.Stdout).With().Timestamp().Str("service", "teamops-api").Str("environment", environment).Logger()
}
