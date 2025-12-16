package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
)

func logBody(ctx context.Context, response *http.Response) {
	l := ctxzap.Extract(ctx)

	if response == nil {
		l.Error("response is nil")
		return
	}

	if response.Body == nil {
		l.Error("response body is nil")
		return
	}

	body := make([]byte, 512)
	_, err := response.Body.Read(body)
	if err != nil {
		l.Error("error reading response body", zap.Error(err))
		return
	}
	l.Info("response body", zap.String("body", string(body)))
}

// Slack API may return errors in the response body even when the HTTP status code is 200.
//
//	extracts a Slack error from a Go error and wraps it with the appropriate gRPC code.
//	This is useful when working with the slack-go library which returns plain Go errors.
func WrapError(err error, contextMsg string) error {
	if err == nil {
		return nil
	}

	grpcCode := MapSlackErrorToGRPCCode(err.Error())
	return uhttp.WrapErrors(grpcCode, contextMsg, err)
}

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// MapSlackErrorToGRPCCode maps Slack error strings to gRPC codes.
func MapSlackErrorToGRPCCode(slackError string) codes.Code {
	lowerError := strings.ToLower(slackError)

	if containsAny(lowerError, "invalid_auth", "token_revoked", "token_expired", "not_authed", "account_inactive") {
		return codes.Unauthenticated
	}

	if containsAny(lowerError, "missing_scope", "access_denied", "not_allowed_token_type", "team_access_not_granted", "no_permission", "ekm_access_denied") {
		return codes.PermissionDenied
	}

	if containsAny(lowerError, "ratelimited") {
		return codes.Unavailable
	}

	if containsAny(lowerError, "user_not_found") {
		return codes.NotFound
	}

	if containsAny(lowerError, "user_already_team_member") {
		return codes.AlreadyExists
	}

	if containsAny(lowerError, "invalid_arguments", "missing_argument", "invalid_arg_name", "invalid_array_arg", "invalid_charset", "invalid_form_data", "invalid_post_type", "missing_post_type", "limit_required") {
		return codes.InvalidArgument
	}

	if containsAny(lowerError, "user_already_deleted", "two_factor_setup_required") {
		return codes.FailedPrecondition
	}

	if containsAny(lowerError, "internal_error", "service_unavailable", "request_timeout") {
		return codes.Unavailable
	}

	if containsAny(lowerError, "fatal_error") {
		return codes.Internal
	}

	if containsAny(lowerError, "method_deprecated", "deprecated_endpoint") {
		return codes.Unimplemented
	}

	return codes.Unknown
}

// Slack API may return errors in the response body even when the HTTP status code is 200.
//
//	examples:
//
//	{"ok":false,"error":"invalid_auth"}
//	{"ok":false,"error": "user_not_found"}
func ErrorWithGrpcCodeFromBytes(bodyBytes []byte) error {
	var baseCheck struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}

	if err := json.Unmarshal(bodyBytes, &baseCheck); err != nil {
		return fmt.Errorf("error parsing Slack API response: %w", err)
	}

	if !baseCheck.Ok && baseCheck.Error != "" {
		grpcCode := MapSlackErrorToGRPCCode(baseCheck.Error)
		return uhttp.WrapErrors(
			grpcCode,
			fmt.Sprintf("Slack API error: %s", baseCheck.Error),
			fmt.Errorf("slack error: %s", baseCheck.Error),
		)
	}

	return nil
}
