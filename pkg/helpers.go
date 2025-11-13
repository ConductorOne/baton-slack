package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/slack-go/slack"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type EnterpriseRolesPagination struct {
	Cursor   string          `json:"cursor"`
	FoundMap map[string]bool `json:"foundMap"`
}

func ParseID(id string) (string, error) {
	parts := strings.Split(id, ":")
	if len(parts) < 2 {
		return "", fmt.Errorf("slack-connector: invalid ID format: %s", id)
	}
	return parts[1], nil
}

func ParseRole(id string) (string, error) {
	parts := strings.Split(id, ":")
	if len(parts) < 3 {
		return "", fmt.Errorf("slack-connector: invalid role ID format: %s", id)
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
			return nil, err
		}
		outputSlice = append(outputSlice, nextResource)
	}
	return outputSlice, nil
}

func (e *EnterpriseRolesPagination) Marshal() (string, error) {
	if e.Cursor == "" {
		return "", nil
	}
	bytes, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("slack-connector: failed to marshal EnterpriseRolesPagination: %w", err)
	}

	return string(bytes), nil
}

func (e *EnterpriseRolesPagination) Unmarshal(input string) error {
	if input == "" {
		e.FoundMap = make(map[string]bool)
		return nil
	}

	err := json.Unmarshal([]byte(input), e)
	if err != nil {
		return fmt.Errorf("slack-connector: failed to unmarshal EnterpriseRolesPagination: %w", err)
	}

	return nil
}

func ParseRolesPageToken(i string) (*EnterpriseRolesPagination, error) {
	b := &EnterpriseRolesPagination{}
	err := b.Unmarshal(i)
	if err != nil {
		return nil, fmt.Errorf("slack-connector: failed to parse roles page token: %w", err)
	}

	if b.FoundMap == nil {
		b.FoundMap = make(map[string]bool)
	}

	return b, nil
}

func ParsePageToken(i string, resourceID *v2.ResourceId) (*pagination.Bag, error) {
	b := &pagination.Bag{}
	err := b.Unmarshal(i)
	if err != nil {
		return nil, fmt.Errorf("slack-connector: pagination bag unmarshal error: %w", err)
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
	switch {
	// Authentication errors (codes.Unauthenticated)
	case strings.Contains(errorString, "token_revoked"),
		strings.Contains(errorString, "token_expired"),
		strings.Contains(errorString, "invalid_auth"),
		strings.Contains(errorString, "not_authed"),
		strings.Contains(errorString, "auth_token_error"),
		strings.Contains(errorString, "invalid_token"):
		return codes.Unauthenticated

	// Authorization/Permission errors (codes.PermissionDenied)
	case strings.Contains(errorString, "missing_scope"),
		strings.Contains(errorString, "missing_scopes"),
		strings.Contains(errorString, "no_permission"),
		strings.Contains(errorString, "access_denied"),
		strings.Contains(errorString, "cannot_"),
		strings.Contains(errorString, "_denied"):
		return codes.PermissionDenied

	// Not Found errors (codes.NotFound)
	case strings.Contains(errorString, "user_not_found"),
		strings.Contains(errorString, "team_not_found"),
		strings.Contains(errorString, "channel_not_found"),
		strings.Contains(errorString, "not_found"),
		strings.Contains(errorString, "app_not_found"),
		strings.Contains(errorString, "workflow_not_found"),
		strings.Contains(errorString, "trigger_not_found"),
		strings.Contains(errorString, "user_already_deleted"):
		return codes.NotFound

	// Invalid Argument errors (codes.InvalidArgument)
	case strings.Contains(errorString, "invalid_arguments"),
		strings.Contains(errorString, "invalid_args"),
		strings.Contains(errorString, "invalid_cursor"),
		strings.Contains(errorString, "invalid_user_id"),
		strings.Contains(errorString, "invalid_channel_id"),
		strings.Contains(errorString, "invalid_"),
		strings.Contains(errorString, "parameter_validation_failed"):
		return codes.InvalidArgument

	// Rate Limiting errors (codes.ResourceExhausted)
	case strings.Contains(errorString, "ratelimited"),
		strings.Contains(errorString, "rate limit"),
		strings.Contains(errorString, "team_quota_exceeded"):
		return codes.ResourceExhausted

	// Service Unavailable errors (codes.Unavailable)
	case strings.Contains(errorString, "503"),
		strings.Contains(errorString, "Service Unavailable"),
		strings.Contains(errorString, "502"),
		strings.Contains(errorString, "Bad Gateway"),
		strings.Contains(errorString, "504"),
		strings.Contains(errorString, "Gateway Timeout"),
		strings.Contains(errorString, "internal_error"),
		strings.Contains(errorString, "http_request_failed"):
		return codes.Unavailable

	// Timeout errors (codes.DeadlineExceeded)
	case strings.Contains(errorString, "timeout"),
		strings.Contains(errorString, "deadline"),
		strings.Contains(errorString, "auth_timeout_error"):
		return codes.DeadlineExceeded

	// Already Exists errors (codes.AlreadyExists)
	case strings.Contains(errorString, "already_exists"),
		strings.Contains(errorString, "app_add_exists"),
		strings.Contains(errorString, "user_already_"),
		strings.Contains(errorString, "user_already_team_member"):
		return codes.AlreadyExists

	// Configuration/Argument errors (codes.InvalidArgument)
	// These are errors where the app/workspace is not properly configured
	case strings.Contains(errorString, "app_not_installed"),
		strings.Contains(errorString, "installation_required"),
		strings.Contains(errorString, "free_team_not_allowed"),
		strings.Contains(errorString, "restricted_plan_level"):
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
	grpcCode := MapSlackErrorToGRPCCode(errMsg)

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

// AnnotationsForError - Intercept ratelimit errors from Slack and create and
// annotation instead.
// TODO(marcos): maybe this should actually still forward along the error.
func AnnotationsForError(err error) (annotations.Annotations, error) {
	annos := annotations.Annotations{}
	var rateLimitErr *slack.RateLimitedError
	if errors.As(err, &rateLimitErr) {
		annos.WithRateLimiting(
			&v2.RateLimitDescription{
				Limit:     0,
				Remaining: 0,
				ResetAt:   timestamppb.New(time.Now().Add(rateLimitErr.RetryAfter)),
			},
		)
		return annos, nil
	}
	// Wrap the error with appropriate gRPC code for non-ratelimit errors
	return annos, WrapSlackClientError(err, "listing resources")
}
