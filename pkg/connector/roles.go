package connector

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-sdk/pkg/uhttp"

	"github.com/conductorone/baton-slack/pkg"
	enterprise "github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
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
	businessPlusClient *enterprise.Client
	enterpriseID     string
}

func (o *workspaceRoleType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func workspaceRoleBuilder(client *slack.Client, enterpriseID string, businessPlusClient *enterprise.Client) *workspaceRoleType {
	return &workspaceRoleType{
		resourceType:     resourceTypeWorkspaceRole,
		client:           client,
		businessPlusClient: businessPlusClient,
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
	found, missing, err := o.businessPlusClient.GetWorkspaceNames(ctx, attrs.Session, []string{resource.ParentResourceId.Resource})
	if err != nil {
		return nil, nil, fmt.Errorf("error getting workspace name for workspace id %s: %w", resource.ParentResourceId.Resource, err)
	}
	workspaceName, exists := found[resource.ParentResourceId.Resource]
	if !exists {
		return nil, nil, fmt.Errorf("workspace not found in cache: %s (missing: %v)", resource.ParentResourceId.Resource, missing)
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
		return nil, uhttp.WrapErrors(codes.InvalidArgument, "only users can be granted workspace role assignments", errors.New("invalid principal type"))
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
	rateLimitData, err = o.businessPlusClient.SetWorkspaceRole(
		ctx,
		teamID,
		principal.Id.Resource,
		roleID,
	)

	outputAnnotations := annotations.New()
	outputAnnotations.WithRateLimiting(rateLimitData)
	if err != nil {
		return outputAnnotations, fmt.Errorf("failed to assign workspace role during grant operation: %w", err)
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
		return nil, uhttp.WrapErrors(codes.InvalidArgument, "enterprise ID and token are both required for workspace role revocation", errors.New("missing enterprise configuration"))
	}

	logger := ctxzap.Extract(ctx)

	principal := grant.Principal

	if principal.Id.ResourceType != resourceTypeUser.Id {
		logger.Warn(
			"baton-slack: only users can have role revoked",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, uhttp.WrapErrors(codes.InvalidArgument, "only users can have workspace role assignments revoked", errors.New("invalid principal type"))
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
		rateLimitData, err = o.businessPlusClient.SetWorkspaceRole(
			ctx,
			teamID,
			principal.Id.Resource,
			RegularRoleID,
		)

	case MemberRoleID:
		rateLimitData, err = o.businessPlusClient.RemoveUser(
			ctx,
			teamID,
			principal.Id.Resource,
		)
	}
	outputAnnotations.WithRateLimiting(rateLimitData)

	if err != nil {
		return outputAnnotations, fmt.Errorf("failed to revoke workspace role during revoke operation: %w", err)
	}

	return outputAnnotations, nil
}
