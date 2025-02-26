package enterprise

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

func logBody(ctx context.Context, response *http.Response) {
	l := ctxzap.Extract(ctx)
	if response == nil {
		l.Error("response is nil")
		return
	}
	bodyCloser := response.Body
	body := make([]byte, 512)
	_, err := bodyCloser.Read(body)
	if err != nil {
		l.Error("error reading response body", zap.Error(err))
		return
	}
	l.Info("response body: ", zap.String("body", string(body)))
}
