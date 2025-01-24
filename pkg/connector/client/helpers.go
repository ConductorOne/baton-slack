package enterprise

import (
	"context"
	"io"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

func logBody(ctx context.Context, bodyCloser io.ReadCloser) {
	l := ctxzap.Extract(ctx)
	body := make([]byte, 512)
	_, err := bodyCloser.Read(body)
	if err != nil {
		l.Error("error reading response body", zap.Error(err))
		return
	}
	l.Info("response body: ", zap.String("body", string(body)))
}
