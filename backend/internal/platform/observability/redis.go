package observability

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type RedisHook struct{}

func NewRedisHook() *RedisHook { return &RedisHook{} }

func (*RedisHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		ctx, span := startRedisSpan(ctx, "dial")
		started := time.Now()
		conn, err := next(ctx, network, addr)
		finishRedisSpan(span, "dial", err, started)
		return conn, err
	}
}

func (*RedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		operation := redisOperation(cmd.Name())
		ctx, span := startRedisSpan(ctx, operation)
		started := time.Now()
		err := next(ctx, cmd)
		finishRedisSpan(span, operation, redisResultError(err), started)
		return err
	}
}

func (*RedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, commands []redis.Cmder) error {
		ctx, span := startRedisSpan(ctx, "pipeline")
		span.SetAttributes(attribute.Int("db.redis.command_count", len(commands)))
		started := time.Now()
		err := next(ctx, commands)
		finishRedisSpan(span, "pipeline", redisResultError(err), started)
		return err
	}
}

func startRedisSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return otel.Tracer("teamops/redis").Start(ctx, "redis."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("db.system.name", "redis"), attribute.String("db.operation.name", operation)),
	)
}

func finishRedisSpan(span trace.Span, operation string, err error, started time.Time) {
	ObserveRedis(operation, err, time.Since(started))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "redis operation failed")
	}
	span.End()
}

func redisResultError(err error) error {
	if errors.Is(err, redis.Nil) {
		return nil
	}
	return err
}
