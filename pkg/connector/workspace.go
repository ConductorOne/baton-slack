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
	"github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/slack-go/slack"
)

const memberEntitlement = "member"

type workspaceResourceType struct {
	resourceType       *v2.ResourceType
	client             *slack.Client
	businessPlusClient *client.Client
}

func (o *workspaceResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func workspaceBuilder(
	slackClient *slack.Client,
	businessPlusClient *client.Client,
) *workspaceResourceType {
	return &workspaceResourceType{
		resourceType:       resourceTypeWorkspace,
		client:             slackClient,
		businessPlusClient: businessPlusClient,
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
		return nil, nil, fmt.Errorf("parsing page token: %w", err)
	}

	var (
		workspaces []slack.Team
		nextCursor string
	)
	params := slack.ListTeamsParameters{Cursor: bag.PageToken()}
	workspaces, nextCursor, err = o.client.ListTeamsContext(ctx, params)
	if err != nil {
		return nil, nil, client.WrapError(err, "error listing teams")
	}

	if o.businessPlusClient != nil {
		err = o.businessPlusClient.SetWorkspaceNames(ctx, attrs.Session, workspaces)
		if err != nil {
			return nil, nil, fmt.Errorf("storing workspace names in session: %w", err)
		}
	}

	rv := make([]*v2.Resource, 0, len(workspaces))
	for _, ws := range workspaces {
		resource, err := workspaceResource(ctx, ws, parentID)
		if err != nil {
			return nil, nil, fmt.Errorf("creating workspace resource: %w", err)
		}
		rv = append(rv, resource)
	}

	err = client.SetWorkspaceNames(ctx, attrs.Session, workspaces)
	if err != nil {
		return nil, nil, fmt.Errorf("storing workspace names in session: %w", err)
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, nil, fmt.Errorf("creating next page token: %w", err)
	}
	return rv, &resources.SyncOpResults{
		NextPageToken: pageToken,
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

// sets workspace memberships and workspace roles
func (o *workspaceResourceType) Grants(
	ctx context.Context,
	resource *v2.Resource,
	attrs resources.SyncOpAttrs,
) ([]*v2.Grant, *resources.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	if o.businessPlusClient == nil {
		l.Debug("Business+ client not available, skipping workspace grants")
		return nil, &resources.SyncOpResults{}, nil
	}

	bag, err := pkg.ParsePageToken(attrs.PageToken.Token, &v2.ResourceId{ResourceType: resourceTypeUser.Id})
	if err != nil {
		return nil, nil, fmt.Errorf("parsing page token: %w", err)
	}

	outputAnnotations := annotations.New()
	users, nextCursor, ratelimitData, err := o.businessPlusClient.GetUsers(
		ctx,
		resource.Id.Resource,
		bag.PageToken(),
	)
	outputAnnotations.WithRateLimiting(ratelimitData)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching users for workspace: %w", err)
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, nil, fmt.Errorf("creating next page token: %w", err)
	}

	var rv []*v2.Grant
	for _, user := range users {
		if user.IsStranger {
			continue
		}
		if user.Deleted {
			continue
		}
		userID, err := resources.NewResourceID(resourceTypeUser, user.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("creating user resource ID: %w", err)
		}

		if user.IsPrimaryOwner {
			rr, err := roleResource(ctx, PrimaryOwnerRoleID, resource.Id)
			if err != nil {
				return nil, nil, fmt.Errorf("creating primary owner role resource: %w", err)
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsOwner {
			rr, err := roleResource(ctx, OwnerRoleID, resource.Id)
			if err != nil {
				return nil, nil, fmt.Errorf("creating owner role resource: %w", err)
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsAdmin {
			rr, err := roleResource(ctx, AdminRoleID, resource.Id)
			if err != nil {
				return nil, nil, fmt.Errorf("creating admin role resource: %w", err)
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsRestricted {
			if user.IsUltraRestricted {
				rr, err := roleResource(ctx, SingleChannelGuestRoleID, resource.Id)
				if err != nil {
					return nil, nil, fmt.Errorf("creating single channel guest role resource: %w", err)
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			} else {
				rr, err := roleResource(ctx, MultiChannelGuestRoleID, resource.Id)
				if err != nil {
					return nil, nil, fmt.Errorf("creating multi channel guest role resource: %w", err)
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			}
		}

		if user.IsInvitedUser {
			rr, err := roleResource(ctx, InvitedMemberRoleID, resource.Id)
			if err != nil {
				return nil, nil, fmt.Errorf("creating invited member role resource: %w", err)
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if !user.IsRestricted && !user.IsUltraRestricted && !user.IsInvitedUser && !user.IsBot && !user.Deleted {
			rr, err := roleResource(ctx, MemberRoleID, resource.Id)
			if err != nil {
				return nil, nil, fmt.Errorf("creating member role resource: %w", err)
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsBot {
			rr, err := roleResource(ctx, BotRoleID, resource.Id)
			if err != nil {
				return nil, nil, fmt.Errorf("creating bot role resource: %w", err)
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		rv = append(rv, grant.NewGrant(resource, memberEntitlement, userID))
	}

	return rv, &resources.SyncOpResults{
		NextPageToken: pageToken,
	}, nil
}

// Grant and Revoke are not implemented for workspace membership because they require
// Enterprise Grid-only API endpoints (admin.users.assign and admin.users.remove).
// These endpoints are only available on Enterprise Grid plans, not Business+ plans.
