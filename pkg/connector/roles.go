package connector

import (
	"context"
	"fmt"
	"maps"
	"slices"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-slack/pkg"
	enterprise "github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
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
	resourceType     *v2.ResourceType
	client           *slack.Client
	enterpriseClient *enterprise.Client
	enterpriseID     string
}

func (o *workspaceRoleType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func workspaceRoleBuilder(client *slack.Client, enterpriseID string, enterpriseClient *enterprise.Client) *workspaceRoleType {
	return &workspaceRoleType{
		resourceType:     resourceTypeWorkspaceRole,
		client:           client,
		enterpriseClient: enterpriseClient,
		enterpriseID:     enterpriseID,
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
	_ *pagination.Token,
) (
	[]*v2.Resource,
	string,
	annotations.Annotations,
	error,
) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	output, err := pkg.MakeResourceList(
		ctx,
		slices.Collect(maps.Keys(roles)),
		parentResourceID,
		roleResource,
	)
	if err != nil {
		return nil, "", nil, err
	}
	return output, "", nil, nil
}

func (o *workspaceRoleType) Entitlements(
	_ context.Context,
	resource *v2.Resource,
	_ *pagination.Token,
) (
	[]*v2.Entitlement,
	string,
	annotations.Annotations,
	error,
) {
	workspaceName, ok := workspacesNameCache[resource.ParentResourceId.Resource]
	if !ok {
		return nil, "", nil, fmt.Errorf("invalid workspace: %s", resource.ParentResourceId.Resource)
	}
	return []*v2.Entitlement{
			entitlement.NewAssignmentEntitlement(
				resource,
				RoleAssignmentEntitlement,
				entitlement.WithGrantableTo(resourceTypeUser),
				entitlement.WithDescription(
					fmt.Sprintf(
						"Has the %s role in the Slack %s workspace",
						resource.DisplayName,
						workspaceName,
					),
				),
				entitlement.WithDisplayName(
					fmt.Sprintf(
						"%s workspace %s role",
						workspaceName,
						resource.DisplayName,
					),
				),
			),
		},
		"",
		nil,
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
	_ *pagination.Token,
) (
	[]*v2.Grant,
	string,
	annotations.Annotations,
	error,
) {
	return nil, "", nil, nil
}

func (o *workspaceRoleType) Grant(
	ctx context.Context,
	principal *v2.Resource,
	entitlement *v2.Entitlement,
) (
	annotations.Annotations,
	error,
) {
	logger := ctxzap.Extract(ctx)

	if principal.Id.ResourceType != resourceTypeUser.Id {
		logger.Warn(
			"baton-slack: only users can be assigned a role",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-slack: only users can be assigned a role")
	}

	// teamID is in the entitlement ID at second position
	teamID, err := pkg.ParseID(entitlement.Id)
	if err != nil {
		return nil, err
	}

	roleID, err := pkg.ParseRole(entitlement.Id)
	if err != nil {
		return nil, err
	}

	var rateLimitData *v2.RateLimitDescription
	rateLimitData, err = o.enterpriseClient.SetWorkspaceRole(
		ctx,
		teamID,
		principal.Id.Resource,
		roleID,
	)

	outputAnnotations := annotations.New()
	outputAnnotations.WithRateLimiting(rateLimitData)
	if err != nil {
		return outputAnnotations, fmt.Errorf("baton-slack: failed to assign user role: %w", err)
	}

	return outputAnnotations, nil
}

func (o *workspaceRoleType) Revoke(
	ctx context.Context,
	grant *v2.Grant,
) (
	annotations.Annotations,
	error,
) {
	if o.enterpriseID == "" {
		return nil, fmt.Errorf("baton-slack: enterprise ID and enterprise token are both required to revoke roles")
	}

	logger := ctxzap.Extract(ctx)

	principal := grant.Principal

	if principal.Id.ResourceType != resourceTypeUser.Id {
		logger.Warn(
			"baton-slack: only users can have role revoked",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-slack: only users can have role revoked")
	}

	// teamID is in the grant ID at second position
	teamID, err := pkg.ParseID(grant.Id)
	if err != nil {
		return nil, err
	}

	role, err := pkg.ParseRole(grant.Id)
	if err != nil {
		return nil, err
	}

	outputAnnotations := annotations.New()

	var rateLimitData *v2.RateLimitDescription
	switch role {
	case AdminRoleID, OwnerRoleID:
		rateLimitData, err = o.enterpriseClient.SetWorkspaceRole(
			ctx,
			teamID,
			principal.Id.Resource,
			RegularRoleID,
		)

	case MemberRoleID:
		rateLimitData, err = o.enterpriseClient.RemoveUser(
			ctx,
			teamID,
			principal.Id.Resource,
		)
	}
	outputAnnotations.WithRateLimiting(rateLimitData)

	if err != nil {
		return outputAnnotations, fmt.Errorf("baton-slack: failed to revoke user role: %w", err)
	}

	return outputAnnotations, nil
}
