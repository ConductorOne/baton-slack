package pkg

import (
	"context"
	"fmt"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"google.golang.org/grpc/codes"
)

func ParseID(id string) (string, error) {
	parts := strings.Split(id, ":")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid ID format: %s", id)
	}
	return parts[1], nil
}

func ParseRole(id string) (string, error) {
	parts := strings.Split(id, ":")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid role ID format: %s", id)
	}
	return parts[2], nil
}

// MakeResourceList - turning arbitrary data into Resource slices is an incredibly common thing.
// TODO(marcos): move to baton-sdk.
func MakeResourceList[T any](
	ctx context.Context,
	objects []T,
	parentResourceID *v2.ResourceId,
	toResource func(
		ctx context.Context,
		object T,
		parentResourceID *v2.ResourceId,
	) (
		*v2.Resource,
		error,
	),
) ([]*v2.Resource, error) {
	outputSlice := make([]*v2.Resource, 0, len(objects))
	for _, object := range objects {
		nextResource, err := toResource(ctx, object, parentResourceID)
		if err != nil {
			return nil, fmt.Errorf("converting object to resource: %w", err)
		}
		outputSlice = append(outputSlice, nextResource)
	}
	return outputSlice, nil
}

func ParsePageToken(i string, resourceID *v2.ResourceId) (*pagination.Bag, error) {
	b := &pagination.Bag{}
	err := b.Unmarshal(i)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling pagination token: %w", err)
	}

	if b.Current() == nil {
		b.Push(pagination.PageState{
			ResourceTypeID: resourceID.ResourceType,
			ResourceID:     resourceID.Resource,
		})
	}

	return b, nil
}

// MapSlackErrorToGRPCCode maps a Slack error string to the appropriate gRPC code.
// This is the single source of truth for Slack error → gRPC code mapping across all clients.
// Error mapping based on Slack API error reference: https://docs.slack.dev/tools/slack-cli/reference/errors/
func MapSlackErrorToGRPCCode(errorString string) codes.Code {
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

	case strings.Contains(lowerErr, "ratelimited"),
		strings.Contains(lowerErr, "rate limit"),
		strings.Contains(lowerErr, "team_quota_exceeded"):
		return codes.DeadlineExceeded

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


