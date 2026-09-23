package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/teamops/backend/internal/app"
	"github.com/example/teamops/backend/internal/cache"
	"github.com/example/teamops/backend/internal/config"
	"github.com/example/teamops/backend/internal/database"
	"github.com/example/teamops/backend/internal/platform/observability"
	"github.com/example/teamops/backend/internal/shared/logger"
)

func main() {
	bootstrapLog := logger.New("startup")
	cfg, err := config.Load()
	if err != nil {
		bootstrapLog.Fatal().Err(err).Msg("configuration startup failed")
	}
	log := logger.New(cfg.Environment)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	traceShutdown, err := observability.SetupTracing(ctx, observability.TraceConfig{
		Endpoint: cfg.OTelEndpoint, Insecure: cfg.OTelInsecure, ServiceName: cfg.OTelServiceName,
		Environment: cfg.Environment, SampleRatio: cfg.OTelSampleRatio,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("tracing startup failed")
	}

	db, err := database.Open(ctx, cfg.DatabaseURL, database.PoolConfig{
		MaxConns:          cfg.DatabaseMaxConns,
		MinConns:          cfg.DatabaseMinConns,
		MaxConnLifetime:   cfg.DatabaseMaxConnLifetime,
		MaxConnIdleTime:   cfg.DatabaseMaxConnIdleTime,
		HealthCheckPeriod: cfg.DatabaseHealthCheckPeriod,
		StatementTimeout:  cfg.DatabaseStatementTimeout,
		LockTimeout:       cfg.DatabaseLockTimeout,
		IdleInTxTimeout:   cfg.DatabaseIdleInTxTimeout,
	}, observability.NewPGXTracer())
	if err != nil {
		log.Fatal().Err(err).Msg("database startup failed")
	}
	cacheClient := cache.New(cfg.RedisAddr, cfg.RedisPassword, observability.NewRedisHook())
	if err = cacheClient.Ping(ctx); err != nil {
		log.Warn().Err(err).Msg("redis unavailable at startup; continuing without cache")
	}

	a := app.New(cfg, db, cacheClient, log)
	a.JobScheduler.Start(ctx)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           a.Router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	serverErrors := make(chan error, 1)
	go func() {
		log.Info().Str("addr", cfg.HTTPAddr).Msg("server listening")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info().Msg("shutdown signal received")
	case err := <-serverErrors:
		log.Error().Err(err).Msg("server stopped unexpectedly")
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed")
		if closeErr := server.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("forced HTTP shutdown failed")
		}
	}
	cancel()
	a.JobScheduler.Stop()
	jobShutdownCtx, cancelJobs := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	if err := a.JobPool.Shutdown(jobShutdownCtx); err != nil {
		log.Error().Err(err).Uint64("audit_events_dropped", a.AuditRecorder.Dropped()).Msg("background job shutdown incomplete")
	}
	if dropped := a.AuditRecorder.Dropped(); dropped > 0 {
		log.Warn().Uint64("dropped", dropped).Msg("audit events were dropped during process lifetime")
	}
	cancelJobs()
	db.Close()
	if err := cacheClient.Close(); err != nil {
		log.Error().Err(err).Msg("redis shutdown failed")
	}
	traceShutdownCtx, cancelTracing := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	if err := traceShutdown(traceShutdownCtx); err != nil {
		log.Error().Err(err).Msg("tracing shutdown failed")
	}
	cancelTracing()
	log.Info().Msg("server stopped")
}
