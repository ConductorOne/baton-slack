package enterprise

import (
	"context"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

// SlackEnterpriseService defines the interface for workspace operations.
type SlackEnterpriseService interface {
	AddUser(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error)
	RemoveUser(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error)
}

// SlackEnterpriseServiceImpl is the default implementation that calls the actual API.
type SlackEnterpriseServiceImpl struct {
	client *Client
}

func NewSlackEnterpriseService(client *Client) SlackEnterpriseService {
	return &SlackEnterpriseServiceImpl{client: client}
}

func (s *SlackEnterpriseServiceImpl) AddUser(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error) {
	return s.client.AddUser(ctx, teamID, userID)
}

func (s *SlackEnterpriseServiceImpl) RemoveUser(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error) {
	return s.client.RemoveUser(ctx, teamID, userID)
}
