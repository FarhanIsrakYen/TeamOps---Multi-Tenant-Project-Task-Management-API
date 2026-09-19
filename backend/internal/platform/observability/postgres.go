package observability

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type queryTraceKey struct{}

type queryTrace struct {
	started   time.Time
	operation string
	span      trace.Span
}

type PGXTracer struct{}

func NewPGXTracer() *PGXTracer { return &PGXTracer{} }

func (*PGXTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	operation := databaseOperation(data.SQL)
	ctx, span := otel.Tracer("teamops/postgresql").Start(ctx, "postgresql."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("db.system.name", "postgresql"), attribute.String("db.operation.name", operation)),
	)
	return context.WithValue(ctx, queryTraceKey{}, queryTrace{started: time.Now(), operation: operation, span: span})
}

func (*PGXTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	state, ok := ctx.Value(queryTraceKey{}).(queryTrace)
	if !ok {
		return
	}
	ObserveDatabase(state.operation, data.Err, time.Since(state.started))
	if data.Err != nil {
		state.span.RecordError(data.Err)
		state.span.SetStatus(codes.Error, "database operation failed")
	}
	state.span.End()
}
