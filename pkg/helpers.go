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
	"github.com/slack-go/slack"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type EnterpriseRolesPagination struct {
	Cursor   string          `json:"cursor"`
	FoundMap map[string]bool `json:"foundMap"`
}

func ParseID(id string) (string, error) {
	parts := strings.Split(id, ":")
	if len(parts) < 2 {
		return "", fmt.Errorf("baton-slack: invalid ID: %s", id)
	}
	return parts[1], nil
}

// MakeResourceList - turning arbitrary data into Resource slices is and
// incredibly common thing. TODO(marcos): move to baton-sdk
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
		return "", err
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
		return err
	}

	return nil
}

func ParseRolesPageToken(i string) (*EnterpriseRolesPagination, error) {
	b := &EnterpriseRolesPagination{}
	err := b.Unmarshal(i)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	if b.Current() == nil {
		b.Push(pagination.PageState{
			ResourceTypeID: resourceID.ResourceType,
			ResourceID:     resourceID.Resource,
		})
	}

	return b, nil
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
	return annos, err
}
