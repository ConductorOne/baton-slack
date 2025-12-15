package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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

// maps Slack error strings to gRPC codes.
func mapSlackErrorToGRPCCode(slackError string) codes.Code {
	switch slackError {
	case "invalid_auth", "token_revoked", "token_expired", "not_authed", "account_inactive":
		return codes.Unauthenticated
	case "missing_scope", "access_denied", "not_allowed_token_type",
		"team_access_not_granted", "no_permission", "ekm_access_denied":
		return codes.PermissionDenied
	case "ratelimited":
		return codes.Unavailable // this would trigger a retry in baton-sdk
	case "user_not_found":
		return codes.NotFound
	case "user_already_team_member":
		return codes.AlreadyExists
	case "invalid_arguments", "missing_argument", "invalid_arg_name",
		"invalid_array_arg", "invalid_charset", "invalid_form_data",
		"invalid_post_type", "missing_post_type", "limit_required":
		return codes.InvalidArgument
	case "user_already_deleted", "two_factor_setup_required":
		return codes.FailedPrecondition
	case "internal_error", "service_unavailable", "request_timeout":
		return codes.Unavailable
	case "fatal_error":
		return codes.Internal
	case "method_deprecated", "deprecated_endpoint":
		return codes.Unimplemented
	default:
		return codes.Unknown
	}
}

// Slack API may return errors in the response body even when the HTTP status code is 200.
// examples:
//
//	{"ok":false,"error":"invalid_auth"}
//	{"ok":false,"error": "user_not_found"}
func checkSlackAPIErrorFromBytes(bodyBytes []byte) error {
	var baseCheck struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}

	if err := json.Unmarshal(bodyBytes, &baseCheck); err != nil {
		return fmt.Errorf("error parsing Slack API response: %w", err)
	}

	if !baseCheck.Ok && baseCheck.Error != "" {
		grpcCode := mapSlackErrorToGRPCCode(baseCheck.Error)
		return uhttp.WrapErrors(
			grpcCode,
			fmt.Sprintf("Slack API error: %s", baseCheck.Error),
			fmt.Errorf("slack error: %s", baseCheck.Error),
		)
	}

	return nil
}
