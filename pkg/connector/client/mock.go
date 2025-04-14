package enterprise

import (
	"context"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

// MockSlackEnterpriseService is a mock implementation of the SlackEnterpriseService interface for testing.
type MockSlackEnterpriseService struct {
	AddUserFunc    func(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error)
	RemoveUserFunc func(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error)
}

func (m *MockSlackEnterpriseService) AddUser(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error) {
	return m.AddUserFunc(ctx, teamID, userID)
}

func (m *MockSlackEnterpriseService) RemoveUser(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error) {
	return m.RemoveUserFunc(ctx, teamID, userID)
}
