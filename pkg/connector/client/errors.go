package client

import (
	"strings"

	"google.golang.org/grpc/codes"
)

func mapSlackErrorToGRPCCode(errorString string) codes.Code {
	lowerErr := strings.ToLower(errorString)

	switch {
	case strings.Contains(lowerErr, "token_revoked"),
		strings.Contains(lowerErr, "token_expired"),
		strings.Contains(lowerErr, "invalid_auth"),
		strings.Contains(lowerErr, "not_authed"),
		strings.Contains(lowerErr, "auth_token_error"),
		strings.Contains(lowerErr, "invalid_token"):
		return codes.Unauthenticated

	case strings.Contains(lowerErr, "missing_scope"),
		strings.Contains(lowerErr, "missing_scopes"),
		strings.Contains(lowerErr, "no_permission"),
		strings.Contains(lowerErr, "access_denied"),
		strings.Contains(lowerErr, "_denied"):
		return codes.PermissionDenied

	case strings.Contains(lowerErr, "user_not_found"),
		strings.Contains(lowerErr, "team_not_found"),
		strings.Contains(lowerErr, "channel_not_found"),
		strings.Contains(lowerErr, "not_found"),
		strings.Contains(lowerErr, "app_not_found"),
		strings.Contains(lowerErr, "workflow_not_found"),
		strings.Contains(lowerErr, "trigger_not_found"),
		strings.Contains(lowerErr, "user_already_deleted"):
		return codes.NotFound

	case strings.Contains(lowerErr, "invalid_arguments"),
		strings.Contains(lowerErr, "invalid_args"),
		strings.Contains(lowerErr, "invalid_cursor"),
		strings.Contains(lowerErr, "invalid_user_id"),
		strings.Contains(lowerErr, "invalid_channel_id"),
		strings.Contains(lowerErr, "invalid_"),
		strings.Contains(lowerErr, "parameter_validation_failed"):
		return codes.InvalidArgument

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

	case strings.Contains(lowerErr, "timeout"),
		strings.Contains(lowerErr, "deadline"),
		strings.Contains(lowerErr, "auth_timeout_error"),
		strings.Contains(lowerErr, "ratelimited"),
		strings.Contains(lowerErr, "rate limit"),
		strings.Contains(lowerErr, "team_quota_exceeded"):
		return codes.DeadlineExceeded

	case strings.Contains(lowerErr, "already_exists"),
		strings.Contains(lowerErr, "app_add_exists"),
		strings.Contains(lowerErr, "user_already_"),
		strings.Contains(lowerErr, "user_already_team_member"):
		return codes.AlreadyExists
	case strings.Contains(lowerErr, "app_not_installed"),
		strings.Contains(lowerErr, "installation_required"),
		strings.Contains(lowerErr, "free_team_not_allowed"),
		strings.Contains(lowerErr, "restricted_plan_level"):
		return codes.InvalidArgument

	default:
		return codes.Unknown
	}
}

func mapSlackErrorToMessage(grpcCode codes.Code) string {
	contextMsg := ""
	switch grpcCode {
	case codes.Unauthenticated:
		contextMsg = "authentication failed"
	case codes.PermissionDenied:
		contextMsg = "insufficient permissions"
	case codes.NotFound:
		contextMsg = "resource not found"
	case codes.InvalidArgument:
		contextMsg = "invalid argument"
	case codes.DeadlineExceeded:
		contextMsg = "rate limited"
	case codes.ResourceExhausted:
		contextMsg = "resource exhausted"
	case codes.Unavailable:
		contextMsg = "service unavailable"
	case codes.AlreadyExists:
		contextMsg = "resource already exists"
	default:
		contextMsg = "error"
	}
	return contextMsg
}
