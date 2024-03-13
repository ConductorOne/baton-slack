package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	enterprise "github.com/conductorone/baton-slack/pkg/slack"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

const (
	PrimaryOwnerRoleID       = "primary_owner"
	OwnerRoleID              = "owner"
	AdminRoleID              = "admin"
	MultiChannelGuestRoleID  = "multi_channel_guest"
	SingleChannelGuestRoleID = "single_channel_guest"
	InvitedMemberRoleID      = "invited_member"
	BotRoleID                = "bot"
	MemberRoleID             = "member"

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
	MemberRoleID:             "Member",
}

type workspaceRoleType struct {
	resourceType     *v2.ResourceType
	client           *slack.Client
	enterpriseClient *enterprise.Client
}

func (o *workspaceRoleType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func workspaceRoleBuilder(client *slack.Client, enterpriseClient *enterprise.Client) *workspaceRoleType {
	return &workspaceRoleType{
		resourceType:     resourceTypeWorkspaceRole,
		client:           client,
		enterpriseClient: enterpriseClient,
	}
}

func roleResource(roleID string, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
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

func (o *workspaceRoleType) List(ctx context.Context, parentResourceID *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

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
	var rv []*v2.Entitlement

	workspaceName, ok := workspacesMap[resource.ParentResourceId.Resource]
	if !ok {
		return nil, "", nil, fmt.Errorf("invalid workspace: %s", resource.ParentResourceId.Resource)
	}

	options := []ent.EntitlementOption{
		ent.WithGrantableTo(resourceTypeUser),
		ent.WithDescription(fmt.Sprintf("Has the %s role in the Slack %s workspace", resource.DisplayName, workspaceName)),
		ent.WithDisplayName(fmt.Sprintf("%s workspace %s role", workspaceName, resource.DisplayName)),
	}

	roleEntitlement := ent.NewAssignmentEntitlement(resource, RoleAssignmentEntitlement, options...)
	rv = append(rv, roleEntitlement)

	return rv, "", nil, nil
}

// Grants would normally return the grants for each role resource. Due to how the Slack API works, it is more efficient to emit these roles while listing
// grants for each individual user. Instead of having to list users for each role we can divine which roles a user should be granted when calculating their grants.
func (o *workspaceRoleType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *workspaceRoleType) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	if principal.Id.ResourceType != resourceTypeUser.Id {
		l.Warn(
			"baton-slack: only users can be assigned a role",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-slack: only users can be assigned a role")
	}

	err := o.enterpriseClient.SetWorkspaceRole(ctx, principal.ParentResourceId.Resource, principal.Id.Resource, entitlement.Resource.Id.Resource)
	if err != nil {
		return nil, fmt.Errorf("baton-slack: failed to assign user role: %w", err)
	}

	return nil, nil
}

func (o *workspaceRoleType) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	principal := grant.Principal

	if principal.Id.ResourceType != resourceTypeUser.Id {
		l.Warn(
			"baton-slack: only users can have role revoked",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-slack: only users can have role revoked")
	}

	// empty role type means regular user
	err := o.enterpriseClient.SetWorkspaceRole(ctx, principal.ParentResourceId.Resource, principal.Id.Resource, "")

	if err != nil {
		return nil, fmt.Errorf("baton-slack: failed to revoke user role: %w", err)
	}

	return nil, nil
}
