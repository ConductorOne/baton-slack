package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/slack-go/slack"
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

// Inspects the error returned by the slack-go library and maps it to appropriate gRPC codes.
func WrapError(err error, contextMsg string) error {
	if err == nil {
		return nil
	}

	// for rate limit errors
	var slackLibrateLimitErr *slack.RateLimitedError
	if errors.As(err, &slackLibrateLimitErr) {
		return uhttp.WrapErrors(codes.Unavailable, contextMsg, err)
	}

	// for 5xx status codes
	var slackLibErr *slack.StatusCodeError
	if errors.As(err, &slackLibErr) {
		grpcCode := httpStatusToGRPCCode(slackLibErr.Code)
		contextMsg = fmt.Sprintf("Slack-go API HTTP error: %s : %s", slackLibErr.Status, contextMsg)
		return uhttp.WrapErrors(grpcCode, contextMsg, err)
	}

	// when ok: false even with 200 HTTP status codes
	var slackErrResp *slack.SlackErrorResponse
	if errors.As(err, &slackErrResp) {
		grpcCode := MapSlackErrorToGRPCCode(slackErrResp.Err)
		if len(slackErrResp.ResponseMetadata.Messages) > 0 {
			contextMsg = fmt.Sprintf("%s (details: %v)", contextMsg, slackErrResp.ResponseMetadata.Messages)
		}
		if len(slackErrResp.ResponseMetadata.Warnings) > 0 {
			contextMsg = fmt.Sprintf("%s (warnings: %v)", contextMsg, slackErrResp.ResponseMetadata.Warnings)
		}
		return uhttp.WrapErrors(grpcCode, contextMsg, err)
	}

	return uhttp.WrapErrors(codes.Unknown, contextMsg, err)
}

func httpStatusToGRPCCode(httpStatus int) codes.Code {
	switch httpStatus {
	case http.StatusBadRequest:
		return codes.Internal
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusConflict:
		return codes.AlreadyExists
	case http.StatusTooManyRequests:
		return codes.Unavailable
	case http.StatusInternalServerError:
		return codes.Internal
	case http.StatusServiceUnavailable:
		return codes.Unavailable
	default:
		return codes.Unknown
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// maps Slack error strings to gRPC codes.
func MapSlackErrorToGRPCCode(slackError string) codes.Code {
	err := strings.ToLower(slackError)

	if containsAny(err, "invalid_auth", "token_revoked", "token_expired", "not_authed", "account_inactive") {
		return codes.Unauthenticated
	}

	if containsAny(err, "missing_scope", "access_denied", "not_allowed_token_type",
		"team_access_not_granted", "no_permission", "ekm_access_denied") {
		return codes.PermissionDenied
	}

	if containsAny(err, "ratelimited") {
		return codes.Unavailable
	}

	if containsAny(err, "user_not_found", "user_already_deleted") {
		return codes.NotFound
	}

	// no_such_subteam is returned when fetching user group members. We determined
	// empirically that retrying on this error allows syncs to complete successfully,
	// so we map it to Unavailable to trigger the SDK's automatic retry logic.
	if containsAny(err, SlackErrNoSuchSubteam) {
		return codes.Unavailable
	}

	if containsAny(err, "user_already_team_member") {
		return codes.AlreadyExists
	}

	if containsAny(err, "invalid_arguments", "missing_argument", "invalid_arg_name",
		"invalid_array_arg", "invalid_charset", "invalid_form_data", "invalid_post_type",
		"missing_post_type", "limit_required") {
		return codes.InvalidArgument
	}

	if containsAny(err, "two_factor_setup_required") {
		return codes.FailedPrecondition
	}

	if containsAny(err, "internal_error", "service_unavailable", "request_timeout") {
		return codes.Unavailable
	}

	if containsAny(err, "fatal_error") {
		return codes.Internal
	}

	if containsAny(err, "method_deprecated", "deprecated_endpoint") {
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
	var res SlackErrorResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return fmt.Errorf("error parsing Slack API response: %w", err)
	}

	if !res.Ok && res.Error != "" {
		grpcCode := MapSlackErrorToGRPCCode(res.Error)
		return uhttp.WrapErrors(
			grpcCode,
			fmt.Sprintf("Slack API error: %s", res.Error),
			fmt.Errorf("slack error: %s", res.Error),
		)
	}

	return nil
}
