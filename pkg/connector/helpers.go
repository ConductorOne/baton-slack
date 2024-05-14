package connector

import (
	"encoding/json"
	"errors"
	"time"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/slack-go/slack"
	"google.golang.org/protobuf/types/known/timestamppb"

	enterprise "github.com/conductorone/baton-slack/pkg/slack"
)

type enterpriseRolesPagination struct {
	Cursor   string          `json:"cursor"`
	FoundMap map[string]bool `json:"foundMap"`
}

func (e *enterpriseRolesPagination) Marshal() (string, error) {
	if e.Cursor == "" {
		return "", nil
	}
	bytes, err := json.Marshal(e)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func (e *enterpriseRolesPagination) Unmarshal(input string) error {
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

func parseRolesPageToken(i string, resourceID *v2.ResourceId) (*enterpriseRolesPagination, error) {
	b := &enterpriseRolesPagination{}
	err := b.Unmarshal(i)
	if err != nil {
		return nil, err
	}

	if b.FoundMap == nil {
		b.FoundMap = make(map[string]bool)
	}

	return b, nil
}

func parsePageToken(i string, resourceID *v2.ResourceId) (*pagination.Bag, error) {
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

func annotationsForError(err error) (annotations.Annotations, error) {
	annos := annotations.Annotations{}
	var rateLimitErr *slack.RateLimitedError
	if errors.As(err, &rateLimitErr) {
		annos.WithRateLimiting(&v2.RateLimitDescription{
			Limit:     0,
			Remaining: 0,
			ResetAt:   timestamppb.New(time.Now().Add(rateLimitErr.RetryAfter)),
		})
		return annos, nil
	}

	var enterpriseRateLimitErr *enterprise.RateLimitError
	if errors.As(err, &enterpriseRateLimitErr) {
		annos.WithRateLimiting(&v2.RateLimitDescription{
			Limit:     0,
			Remaining: 0,
			ResetAt:   timestamppb.New(time.Now().Add(enterpriseRateLimitErr.RetryAfter)),
		})
		return annos, nil
	}

	return annos, err
}
