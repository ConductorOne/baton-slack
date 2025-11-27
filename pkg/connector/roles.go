package connector

import (
	"context"
	"fmt"
	"maps"
	"slices"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"

	"github.com/conductorone/baton-slack/pkg"
	"github.com/slack-go/slack"
)

const (
	PrimaryOwnerRoleID        = "primary_owner"
	OwnerRoleID               = "owner"
	AdminRoleID               = "admin"
	MultiChannelGuestRoleID   = "multi_channel_guest"
	SingleChannelGuestRoleID  = "single_channel_guest"
	InvitedMemberRoleID       = "invited_member"
	BotRoleID                 = "bot"
	MemberRoleID              = "member"
	RoleAssignmentEntitlement = "assigned"
	// empty role type means regular user.
	RegularRoleID = ""
)

var roles = map[string]string{
	PrimaryOwnerRoleID:       "Primary Owner",
	OwnerRoleID:              "Owner",
	AdminRoleID:              "Admin",
	MultiChannelGuestRoleID:  "Multi Channel Guest",
	SingleChannelGuestRoleID: "Single Channel Guest",
	InvitedMemberRoleID:      "Invited member",
	BotRoleID:                "Bot",
	MemberRoleID:             "Member",
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

func roleResource(
	_ context.Context,
	roleID string,
	parentResourceID *v2.ResourceId,
) (*v2.Resource, error) {
	roleName, ok := roles[roleID]
	if !ok {
		return nil, fmt.Errorf("invalid roleID: %s", roleID)
	}

	roleId := fmt.Sprintf("%s:%s", parentResourceID.Resource, roleID)

	r, err := resources.NewRoleResource(
		roleName,
		resourceTypeWorkspaceRole,
		roleId,
		nil,
		resources.WithParentResourceID(parentResourceID))
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (o *workspaceRoleType) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	_ resources.SyncOpAttrs,
) (
	[]*v2.Resource,
	*resources.SyncOpResults,
	error,
) {
	if parentResourceID == nil {
		return nil, &resources.SyncOpResults{}, nil
	}

	output, err := pkg.MakeResourceList(
		ctx,
		slices.Collect(maps.Keys(roles)),
		parentResourceID,
		roleResource,
	)
	if err != nil {
		return nil, nil, err
	}
	return output, &resources.SyncOpResults{}, nil
}

func (o *workspaceRoleType) Entitlements(
	ctx context.Context,
	resource *v2.Resource,
	attrs resources.SyncOpAttrs,
) (
	[]*v2.Entitlement,
	*resources.SyncOpResults,
	error,
) {
	return []*v2.Entitlement{
			entitlement.NewAssignmentEntitlement(
				resource,
				RoleAssignmentEntitlement,
				entitlement.WithGrantableTo(resourceTypeUser),
				entitlement.WithDescription(
					fmt.Sprintf(
						"Has the %s role in the Slack workspace",
						resource.DisplayName,
					),
				),
				entitlement.WithDisplayName(
					fmt.Sprintf(
						"%s role",
						resource.DisplayName,
					),
				),
			),
		},
		&resources.SyncOpResults{},
		nil
}

// Grants would normally return the grants for each role resource. Due to how
// the Slack API works, it is more efficient to emit these roles while listing
// grants for each individual user. Instead of having to list users for each
// role we can divine which roles a user should be granted when calculating
// their grants.
func (o *workspaceRoleType) Grants(
	_ context.Context,
	_ *v2.Resource,
	_ resources.SyncOpAttrs,
) (
	[]*v2.Grant,
	*resources.SyncOpResults,
	error,
) {
	return nil, &resources.SyncOpResults{}, nil
}
