package jobs

import (
	"context"
	"github.com/rs/zerolog"
	"time"
)

type SessionCleaner interface {
	DeleteExpiredSessions(context.Context) (int64, error)
}

func RunSessionCleanup(ctx context.Context, repo SessionCleaner, log zerolog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := repo.DeleteExpiredSessions(ctx)
			if err != nil {
				log.Error().Err(err).Msg("session cleanup failed")
			} else if n > 0 {
				log.Info().Int64("deleted", n).Msg("expired sessions cleaned")
			}
		}
	}
}
