package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/slack-go/slack"
)

const (
	PrimaryOwnerRoleID       = "primary_owner"
	OwnerRoleID              = "owner"
	AdminRoleID              = "admin"
	MultiChannelGuestRoleID  = "multi_channel_guest"
	SingleChannelGuestRoleID = "single_channel_guest"
	InvitedMemberRoleID      = "invited_member"
	BotRoleID                = "bot"

	RoleAssignmentEntitlement = "assigned"
)

var roles = map[string]string{
	PrimaryOwnerRoleID:       "Primary Owner",
	OwnerRoleID:              "Owner",
	AdminRoleID:              "Admin",
	MultiChannelGuestRoleID:  "Multi Channel Guest",
	SingleChannelGuestRoleID: "Single Channel Guest",
	InvitedMemberRoleID:      "Invited member",
	BotRoleID:                "Bot",
}

type workspaceRoleType struct {
	resourceType *v2.ResourceType
	client       *slack.Client
}

func (o *workspaceRoleType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func workspaceRoleBuilder(client *slack.Client) *workspaceRoleType {
	return &workspaceRoleType{
		resourceType: resourceTypeWorkspaceRole,
		client:       client,
	}
}

func roleResource(roleID string, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	roleName, ok := roles[roleID]
	if !ok {
		return nil, fmt.Errorf("invalid roleID: %s", roleID)
	}

	r, err := resources.NewRoleResource(
		roleName,
		resourceTypeWorkspaceRole,
		roleID,
		nil,
		resources.WithParentResourceID(parentResourceID))
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (o *workspaceRoleType) List(ctx context.Context, parentResourceID *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var ret []*v2.Resource

	for roleID := range roles {
		r, err := roleResource(roleID, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}

		ret = append(ret, r)
	}

	return ret, "", nil, nil
}

func (o *workspaceRoleType) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	rv := []*v2.Entitlement{
		ent.NewAssignmentEntitlement(
			resource,
			RoleAssignmentEntitlement,
			ent.WithGrantableTo(resourceTypeUser),
			ent.WithDescription(fmt.Sprintf("Has the %s role in the Slack workspace", resource.DisplayName)),
			ent.WithDisplayName(fmt.Sprintf("%s Workspace Role", resource.DisplayName)),
		),
	}

	return rv, "", nil, nil
}

// Grants would normally return the grants for each role resource. Due to how the Slack API works, it is more efficient to emit these roles while listing
// grants for each individual user. Instead of having to list users for each role we can divine which roles a user should be granted when calculating their grants.
func (o *workspaceRoleType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}
