package audit

import "context"

type RequestInfo struct {
	RequestID string
	IPAddress string
	UserAgent string
}

type requestInfoKey struct{}

func ContextWithRequestInfo(ctx context.Context, info RequestInfo) context.Context {
	return context.WithValue(ctx, requestInfoKey{}, info)
}

func RequestInfoFromContext(ctx context.Context) RequestInfo {
	info, _ := ctx.Value(requestInfoKey{}).(RequestInfo)
	return info
}
