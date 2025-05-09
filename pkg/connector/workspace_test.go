package connector

import (
	"context"
	"fmt"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	enterprise "github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test builder with mocks.
func newTestWorkspaceBuilder() (*workspaceResourceType, *enterprise.MockSlackEnterpriseService) {
	mockEnterpriseClient := enterprise.Client{}
	mockSlackClient := slack.Client{}
	mockEnterpriseService := &enterprise.MockSlackEnterpriseService{}

	builder := workspaceBuilder(&mockSlackClient, "test-enterprise", &mockEnterpriseClient)

	// Replace the Enterprise service with our mock.
	builder.enterpriseService = mockEnterpriseService

	return builder, mockEnterpriseService
}

func TestWorkspaceGrantAndRevoke(t *testing.T) {
	ctx := context.Background()

	t.Run("Grant operation for workspace with valid principal and entitlement", func(t *testing.T) {
		workspaceBuilder, mockService := newTestWorkspaceBuilder()
		// Mock the add user to workspace call.
		mockService.AddUserFunc = func(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error) {
			assert.Equal(t, "test-team", teamID)
			assert.Equal(t, "test-user", userID)
			return nil, nil
		}

		// Create a grant request.
		principal := &v2.Resource{
			Id: &v2.ResourceId{
				ResourceType: resourceTypeUser.Id,
				Resource:     "test-user",
			},
		}

		entitlement := &v2.Entitlement{
			Resource: &v2.Resource{
				Id: &v2.ResourceId{
					Resource: "test-team",
				},
			},
		}

		// Execute Grant.
		_, err := workspaceBuilder.Grant(ctx, principal, entitlement)

		// Verify the result.
		require.NoError(t, err)
	})

	t.Run("Grant operation for workspace with invalid principal", func(t *testing.T) {
		workspaceBuilder, _ := newTestWorkspaceBuilder()

		principal := &v2.Resource{
			Id: &v2.ResourceId{
				ResourceType: "invalid-type",
				Resource:     "test-user",
			},
		}

		entitlement := &v2.Entitlement{
			Resource: &v2.Resource{
				Id: &v2.ResourceId{
					Resource: "test-team",
				},
			},
		}

		// Execute Grant.
		_, err := workspaceBuilder.Grant(ctx, principal, entitlement)

		// Verify the error.
		require.Error(t, err)
		assert.Contains(t, err.Error(), "baton-slack: only users can be assigned to a workspace")
	})

	t.Run("Grant operation for workspace with already granted principal", func(t *testing.T) {
		workspaceBuilder, mockService := newTestWorkspaceBuilder()
		// Mock the add user to workspace call.
		mockService.AddUserFunc = func(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error) {
			return nil, fmt.Errorf(enterprise.SlackErrUserAlreadyTeamMember)
		}

		principal := &v2.Resource{
			Id: &v2.ResourceId{
				ResourceType: resourceTypeUser.Id,
				Resource:     "test-user",
			},
		}

		entitlement := &v2.Entitlement{
			Resource: &v2.Resource{
				Id: &v2.ResourceId{
					Resource: "test-team",
				},
			},
		}

		// Execute Grant.
		annotations, err := workspaceBuilder.Grant(ctx, principal, entitlement)

		// Verify the result.
		require.NoError(t, err)
		require.True(t, annotations.Contains(&v2.GrantAlreadyExists{}))
	})

	t.Run("Revoke operation for workspace with valid principal and entitlement", func(t *testing.T) {
		workspaceBuilder, mockService := newTestWorkspaceBuilder()
		// Mock the remove user from workspace call.
		mockService.RemoveUserFunc = func(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error) {
			assert.Equal(t, "test-team", teamID)
			assert.Equal(t, "test-user", userID)
			return nil, nil
		}

		principal := &v2.Resource{
			Id: &v2.ResourceId{
				ResourceType: resourceTypeUser.Id,
				Resource:     "test-user",
			},
		}

		entitlement := &v2.Entitlement{
			Resource: &v2.Resource{
				Id: &v2.ResourceId{
					Resource: "test-team",
				},
			},
		}

		grant := &v2.Grant{
			Principal:   principal,
			Entitlement: entitlement,
		}

		// Execute Revoke.
		_, err := workspaceBuilder.Revoke(ctx, grant)

		// Verify the result.
		require.NoError(t, err)
	})

	t.Run("Revoke operation for workspace with invalid principal", func(t *testing.T) {
		workspaceBuilder, _ := newTestWorkspaceBuilder()

		principal := &v2.Resource{
			Id: &v2.ResourceId{
				ResourceType: "invalid-type",
				Resource:     "test-user",
			},
		}

		entitlement := &v2.Entitlement{
			Resource: &v2.Resource{
				Id: &v2.ResourceId{
					Resource: "test-team",
				},
			},
		}

		grant := &v2.Grant{
			Principal:   principal,
			Entitlement: entitlement,
		}

		// Execute Revoke.
		_, err := workspaceBuilder.Revoke(ctx, grant)

		// Verify the error.
		require.Error(t, err)
		assert.Contains(t, err.Error(), "baton-slack: only users can be revoked from a workspace")
	})

	t.Run("Revoke operation for workspace with already revoked principal", func(t *testing.T) {
		workspaceBuilder, mockService := newTestWorkspaceBuilder()
		// Mock the remove user from workspace call.
		mockService.RemoveUserFunc = func(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error) {
			return nil, fmt.Errorf(enterprise.SlackErrUserAlreadyDeleted)
		}

		principal := &v2.Resource{
			Id: &v2.ResourceId{
				ResourceType: resourceTypeUser.Id,
				Resource:     "test-user",
			},
		}

		entitlement := &v2.Entitlement{
			Resource: &v2.Resource{
				Id: &v2.ResourceId{
					Resource: "test-team",
				},
			},
		}

		grant := &v2.Grant{
			Principal:   principal,
			Entitlement: entitlement,
		}

		// Execute Revoke.
		annotations, err := workspaceBuilder.Revoke(ctx, grant)

		// Verify the result.
		require.NoError(t, err)
		require.True(t, annotations.Contains(&v2.GrantAlreadyRevoked{}))
	})
}
