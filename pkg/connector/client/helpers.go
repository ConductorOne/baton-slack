package enterprise

import (
	"context"
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
	bodyCloser := response.Body
	body := make([]byte, 512)
	_, err := bodyCloser.Read(body)
	if err != nil {
		l.Error("error reading response body", zap.Error(err))
		return
	}
	l.Info("response body: ", zap.String("body", string(body)))
}

// mapSlackErrorToGRPCCode maps a Slack error string to the appropriate gRPC code.
// This is the single source of truth for Slack error → gRPC code mapping across all clients.
// Error mapping based on Slack API error reference: https://docs.slack.dev/tools/slack-cli/reference/errors/
func mapSlackErrorToGRPCCode(errorString string) codes.Code {
	// Normalize to lowercase since Slack returns error identifiers in lowercase with underscores
	lowerErr := strings.ToLower(errorString)

	switch {
	// Authentication errors (codes.Unauthenticated)
	case strings.Contains(lowerErr, "token_revoked"),
		strings.Contains(lowerErr, "token_expired"),
		strings.Contains(lowerErr, "invalid_auth"),
		strings.Contains(lowerErr, "not_authed"),
		strings.Contains(lowerErr, "auth_token_error"),
		strings.Contains(lowerErr, "invalid_token"):
		return codes.Unauthenticated

	// Authorization/Permission errors (codes.PermissionDenied)
	case strings.Contains(lowerErr, "missing_scope"),
		strings.Contains(lowerErr, "missing_scopes"),
		strings.Contains(lowerErr, "no_permission"),
		strings.Contains(lowerErr, "access_denied"),
		strings.Contains(lowerErr, "_denied"):
		return codes.PermissionDenied

	// Not Found errors (codes.NotFound)
	case strings.Contains(lowerErr, "user_not_found"),
		strings.Contains(lowerErr, "team_not_found"),
		strings.Contains(lowerErr, "channel_not_found"),
		strings.Contains(lowerErr, "not_found"),
		strings.Contains(lowerErr, "app_not_found"),
		strings.Contains(lowerErr, "workflow_not_found"),
		strings.Contains(lowerErr, "trigger_not_found"),
		strings.Contains(lowerErr, "user_already_deleted"):
		return codes.NotFound

	// Invalid Argument errors (codes.InvalidArgument)
	case strings.Contains(lowerErr, "invalid_arguments"),
		strings.Contains(lowerErr, "invalid_args"),
		strings.Contains(lowerErr, "invalid_cursor"),
		strings.Contains(lowerErr, "invalid_user_id"),
		strings.Contains(lowerErr, "invalid_channel_id"),
		strings.Contains(lowerErr, "invalid_"),
		strings.Contains(lowerErr, "parameter_validation_failed"):
		return codes.InvalidArgument

	// Rate Limiting errors (codes.ResourceExhausted)
	case strings.Contains(lowerErr, "ratelimited"),
		strings.Contains(lowerErr, "rate limit"),
		strings.Contains(lowerErr, "team_quota_exceeded"):
		return codes.ResourceExhausted

	// Service Unavailable errors (codes.Unavailable)
	case strings.Contains(lowerErr, "503"),
		strings.Contains(lowerErr, "service_unavailable"),
		strings.Contains(lowerErr, "service unavailable"),
		strings.Contains(lowerErr, "502"),
		strings.Contains(lowerErr, "bad_gateway"),
		strings.Contains(lowerErr, "bad gateway"),
		strings.Contains(lowerErr, "504"),
		strings.Contains(lowerErr, "gateway_timeout"),
		strings.Contains(lowerErr, "gateway timeout"),
		strings.Contains(lowerErr, "internal_error"),
		strings.Contains(lowerErr, "http_request_failed"):
		return codes.Unavailable

	// Timeout errors (codes.DeadlineExceeded)
	case strings.Contains(lowerErr, "timeout"),
		strings.Contains(lowerErr, "deadline"),
		strings.Contains(lowerErr, "auth_timeout_error"):
		return codes.DeadlineExceeded

	// Already Exists errors (codes.AlreadyExists)
	case strings.Contains(lowerErr, "already_exists"),
		strings.Contains(lowerErr, "app_add_exists"),
		strings.Contains(lowerErr, "user_already_"),
		strings.Contains(lowerErr, "user_already_team_member"):
		return codes.AlreadyExists

	// Configuration/Argument errors (codes.InvalidArgument)
	// These are errors where the app/workspace is not properly configured
	case strings.Contains(lowerErr, "app_not_installed"),
		strings.Contains(lowerErr, "installation_required"),
		strings.Contains(lowerErr, "free_team_not_allowed"),
		strings.Contains(lowerErr, "restricted_plan_level"):
		return codes.InvalidArgument

	default:
		// For unknown errors, use Unknown code
		return codes.Unknown
	}
}

// WrapSlackClientError wraps errors from the slack-go/slack client with appropriate
// gRPC codes for better classification and handling in C1 and alerting systems.
func WrapSlackClientError(err error, action string) error {
	if err == nil {
		return nil
	}

	// Check for rate limit errors (slack.RateLimitedError type)
	var rateLimitErr *slack.RateLimitedError
	if errors.As(err, &rateLimitErr) {
		return uhttp.WrapErrors(
			codes.ResourceExhausted,
			fmt.Sprintf("rate limited during %s", action),
			err,
		)
	}

	// Map the error string to a gRPC code using the centralized mapping function
	errMsg := err.Error()
	grpcCode := mapSlackErrorToGRPCCode(errMsg)

	// Create appropriate error message based on the code
	var contextMsg string
	switch grpcCode {
	case codes.Unauthenticated:
		contextMsg = "authentication failed"
	case codes.PermissionDenied:
		contextMsg = "insufficient permissions"
	case codes.NotFound:
		contextMsg = "resource not found"
	case codes.InvalidArgument:
		contextMsg = "invalid argument"
	case codes.ResourceExhausted:
		contextMsg = "rate limited"
	case codes.Unavailable:
		contextMsg = "service unavailable"
	case codes.DeadlineExceeded:
		contextMsg = "timeout"
	case codes.AlreadyExists:
		contextMsg = "resource already exists"
	case codes.FailedPrecondition:
		contextMsg = "precondition failed"
	default:
		contextMsg = "error"
	}

	return uhttp.WrapErrors(
		grpcCode,
		fmt.Sprintf("%s during %s", contextMsg, action),
		err,
	)
}

// handleError - Slack can return a 200 with an error in the JSON body.
// This function wraps errors with appropriate gRPC codes for better classification
// and handling in C1 and alerting systems.
// It uses the centralized mapSlackErrorToGRPCCode function from pkg/helpers.go.
func (a *BaseResponse) handleError(err error, action string) error {
	if err != nil {
		return fmt.Errorf("error %s: %w", action, err)
	}

	if a.Error != "" {
		// Use the centralized error mapping from pkg package
		grpcCode := mapSlackErrorToGRPCCode(a.Error)

		// Build detailed error message
		errMsg := a.Error
		if a.Needed != "" || a.Provided != "" {
			errMsg = fmt.Sprintf("%s (needed: %v, provided: %v)", a.Error, a.Needed, a.Provided)
		}

		// Create appropriate context message based on the code
		var contextMsg string
		switch grpcCode {
		case codes.Unauthenticated:
			contextMsg = "authentication failed"
		case codes.PermissionDenied:
			contextMsg = "insufficient permissions"
		case codes.NotFound:
			contextMsg = "resource not found"
		case codes.InvalidArgument:
			contextMsg = "invalid argument"
		case codes.ResourceExhausted:
			contextMsg = "rate limited"
		case codes.Unavailable:
			contextMsg = "service unavailable"
		case codes.AlreadyExists:
			contextMsg = "resource already exists"
		default:
			contextMsg = "error"
		}

		return uhttp.WrapErrors(
			grpcCode,
			fmt.Sprintf("%s during %s", contextMsg, action),
			errors.New(errMsg),
		)
	}
	return nil
}

func (a *SCIMResponse[T]) handleError(err error, action string) error {
	if err != nil {
		return fmt.Errorf("error %s: %w", action, err)
	}

	return nil
}

func (a *UserResource) handleError(err error, action string) error {
	if err != nil {
		return fmt.Errorf("error %s: %w", action, err)
	}

	return nil
}

func (a *GroupResource) handleError(err error, action string) error {
	if err != nil {
		return fmt.Errorf("error %s: %w", action, err)
	}

	return nil
}