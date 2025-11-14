package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-slack/pkg"
	enterprise "github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

const memberEntitlement = "member"

type workspaceResourceType struct {
	resourceType      *v2.ResourceType
	client            *slack.Client
	enterpriseID      string
	enterpriseService enterprise.SlackEnterpriseService
	enterpriseClient  *enterprise.Client
}

func (o *workspaceResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func workspaceBuilder(
	client *slack.Client,
	enterpriseID string,
	enterpriseClient *enterprise.Client,
) *workspaceResourceType {
	return &workspaceResourceType{
		resourceType:      resourceTypeWorkspace,
		client:            client,
		enterpriseID:      enterpriseID,
		enterpriseClient:  enterpriseClient,
		enterpriseService: enterprise.NewSlackEnterpriseService(enterpriseClient),
	}
}

// Create a new connector resource for a Slack workspace.
func workspaceResource(
	_ context.Context,
	workspace slack.Team,
	_ *v2.ResourceId,
) (*v2.Resource, error) {
	return resources.NewGroupResource(
		workspace.Name,
		resourceTypeWorkspace,
		workspace.ID,
		[]resources.GroupTraitOption{
			resources.WithGroupProfile(
				map[string]interface{}{
					"workspace_id":     workspace.ID,
					"workspace_name":   workspace.Name,
					"workspace_domain": workspace.Domain,
				},
			),
		},
		resources.WithAnnotation(
			&v2.ChildResourceType{ResourceTypeId: resourceTypeUser.Id},
			&v2.ChildResourceType{ResourceTypeId: resourceTypeUserGroup.Id},
			&v2.ChildResourceType{ResourceTypeId: resourceTypeWorkspaceRole.Id},
		),
	)
}

func (o *workspaceResourceType) List(
	ctx context.Context,
	parentID *v2.ResourceId,
	attrs resources.SyncOpAttrs,
) ([]*v2.Resource, *resources.SyncOpResults, error) {
	bag, err := pkg.ParsePageToken(attrs.PageToken.Token, &v2.ResourceId{ResourceType: resourceTypeWorkspace.Id})
	if err != nil {
		return nil, nil, err
	}

	var (
		workspaces    []slack.Team
		nextCursor    string
		ratelimitData *v2.RateLimitDescription
	)
	outputAnnotations := annotations.New()
	if o.enterpriseID != "" {
		workspaces, nextCursor, ratelimitData, err = o.enterpriseClient.GetAuthTeamsList(ctx, bag.PageToken())
		outputAnnotations.WithRateLimiting(ratelimitData)
		if err != nil {
			return nil, &resources.SyncOpResults{Annotations: outputAnnotations}, err
		}
	} else {
		params := slack.ListTeamsParameters{Cursor: bag.PageToken()}
		workspaces, nextCursor, err = o.client.ListTeamsContext(ctx, params)
		if err != nil {
			return nil, nil, err
		}
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, nil, err
	}

	err = o.enterpriseClient.SetWorkspaceNames(ctx, attrs.Session, workspaces)
	if err != nil {
		return nil, nil, err
	}

	output, err := pkg.MakeResourceList(
		ctx,
		workspaces,
		nil,
		workspaceResource,
	)
	if err != nil {
		return nil, nil, err
	}

	return output, &resources.SyncOpResults{
		NextPageToken: pageToken,
		Annotations:   outputAnnotations,
	}, nil
}

func (o *workspaceResourceType) Entitlements(
	_ context.Context,
	resource *v2.Resource,
	attrs resources.SyncOpAttrs,
) ([]*v2.Entitlement, *resources.SyncOpResults, error) {
	return []*v2.Entitlement{
		entitlement.NewAssignmentEntitlement(
			resource,
			memberEntitlement,
			entitlement.WithGrantableTo(resourceTypeUser),
			entitlement.WithDescription(
				fmt.Sprintf(
					"Member of the %s workspace",
					resource.DisplayName,
				),
			),
			entitlement.WithDisplayName(
				fmt.Sprintf(
					"%s workspace member",
					resource.DisplayName,
				),
			),
		),
	}, &resources.SyncOpResults{}, nil
}

func (o *workspaceResourceType) Grants(
	ctx context.Context,
	resource *v2.Resource,
	attrs resources.SyncOpAttrs,
) ([]*v2.Grant, *resources.SyncOpResults, error) {
	bag, err := pkg.ParsePageToken(attrs.PageToken.Token, &v2.ResourceId{ResourceType: resourceTypeUser.Id})
	if err != nil {
		return nil, nil, err
	}

	outputAnnotations := annotations.New()
	users, nextCursor, ratelimitData, err := o.enterpriseClient.GetUsers(
		ctx,
		resource.Id.Resource,
		bag.PageToken(),
	)
	outputAnnotations.WithRateLimiting(ratelimitData)
	if err != nil {
		return nil, nil, err
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, nil, err
	}

	var rv []*v2.Grant
	for _, user := range users {
		if user.IsStranger {
			continue
		}
		userID, err := resources.NewResourceID(resourceTypeUser, user.ID)
		if err != nil {
			return nil, nil, err
		}

		if user.IsPrimaryOwner {
			rr, err := roleResource(ctx, PrimaryOwnerRoleID, resource.Id)
			if err != nil {
				return nil, nil, err
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsOwner {
			rr, err := roleResource(ctx, OwnerRoleID, resource.Id)
			if err != nil {
				return nil, nil, err
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsAdmin {
			rr, err := roleResource(ctx, AdminRoleID, resource.Id)
			if err != nil {
				return nil, nil, err
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsRestricted {
			if user.IsUltraRestricted {
				rr, err := roleResource(ctx, SingleChannelGuestRoleID, resource.Id)
				if err != nil {
					return nil, nil, err
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			} else {
				rr, err := roleResource(ctx, MultiChannelGuestRoleID, resource.Id)
				if err != nil {
					return nil, nil, err
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			}
		}

		if user.IsInvitedUser {
			rr, err := roleResource(ctx, InvitedMemberRoleID, resource.Id)
			if err != nil {
				return nil, nil, err
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if !user.IsRestricted && !user.IsUltraRestricted && !user.IsInvitedUser && !user.IsBot && !user.Deleted {
			rr, err := roleResource(ctx, MemberRoleID, resource.Id)
			if err != nil {
				return nil, nil, err
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsBot {
			rr, err := roleResource(ctx, BotRoleID, resource.Id)
			if err != nil {
				return nil, nil, err
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if o.enterpriseID != "" {
			if user.Enterprise.IsPrimaryOwner {
				rr, err := enterpriseRoleResource(ctx, OrganizationPrimaryOwnerID, resource.Id)
				if err != nil {
					return nil, nil, err
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			}
			if user.Enterprise.IsOwner {
				rr, err := enterpriseRoleResource(ctx, OrganizationOwnerID, resource.Id)
				if err != nil {
					return nil, nil, err
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			}
			if user.Enterprise.IsAdmin {
				rr, err := enterpriseRoleResource(ctx, OrganizationAdminID, resource.Id)
				if err != nil {
					return nil, nil, err
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			}
		}

		// confused about Workspace vs Workspace Role? check this link:
		// https://github.com/ConductorOne/baton-slack/pull/4
		rv = append(rv, grant.NewGrant(resource, memberEntitlement, userID))
	}

	return rv, &resources.SyncOpResults{
		NextPageToken: pageToken,
	}, nil
}

func (o *workspaceResourceType) Grant(
	ctx context.Context,
	principal *v2.Resource,
	entitlement *v2.Entitlement,
) (annotations.Annotations, error) {
	if o.enterpriseID == "" {
		return nil, fmt.Errorf("baton-slack: enterprise ID and enterprise token are both required")
	}

	logger := ctxzap.Extract(ctx)

	if principal.Id.ResourceType != resourceTypeUser.Id {
		logger.Warn(
			"baton-slack: only users can be assigned to a workspace",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-slack: only users can be assigned to a workspace")
	}

	outputAnnotations := annotations.New()

	// Add the user to the workspace directly without requiring confirmation
	rateLimitData, err := o.enterpriseService.AddUser(
		ctx,
		entitlement.Resource.Id.Resource,
		principal.Id.Resource,
	)
	outputAnnotations.WithRateLimiting(rateLimitData)

	if err != nil {
		// Check if the error indicates the user is already a member.
		if err.Error() == enterprise.SlackErrUserAlreadyTeamMember {
			outputAnnotations.Append(&v2.GrantAlreadyExists{})
			return outputAnnotations, nil
		}
		// Handle other errors.
		return outputAnnotations, fmt.Errorf("baton-slack: failed to add user to workspace: %w", err)
	}

	return outputAnnotations, nil
}

func (o *workspaceResourceType) Revoke(
	ctx context.Context,
	grant *v2.Grant,
) (
	annotations.Annotations,
	error,
) {
	if o.enterpriseID == "" {
		return nil, fmt.Errorf("baton-slack: enterprise ID and enterprise token are both required to revoke grants")
	}

	logger := ctxzap.Extract(ctx)

	principal := grant.Principal
	if principal.Id.ResourceType != resourceTypeUser.Id {
		logger.Warn(
			"baton-slack: only users can be revoked from a workspace",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-slack: only users can be revoked from a workspace")
	}

	outputAnnotations := annotations.New()

	// Remove the user from the workspace directly without requiring confirmation
	rateLimitData, err := o.enterpriseService.RemoveUser(
		ctx,
		grant.Entitlement.Resource.Id.Resource,
		principal.Id.Resource,
	)
	outputAnnotations.WithRateLimiting(rateLimitData)

	if err != nil {
		// Check if the error indicates the user is already deleted/removed.
		if err.Error() == enterprise.SlackErrUserAlreadyDeleted {
			outputAnnotations.Append(&v2.GrantAlreadyRevoked{})
			return outputAnnotations, nil
		}
		// Handle other errors.
		return outputAnnotations, fmt.Errorf("baton-slack: failed to remove user from workspace: %w", err)
	}

	return outputAnnotations, nil
}
