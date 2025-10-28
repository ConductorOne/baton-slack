package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/conductorone/baton-slack/pkg"
	enterprise "github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
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
	_ *v2.ResourceId,
	pt *pagination.Token,
) (
	[]*v2.Resource,
	string,
	annotations.Annotations,
	error,
) {
	bag, err := pkg.ParsePageToken(pt.Token, &v2.ResourceId{ResourceType: resourceTypeWorkspace.Id})
	if err != nil {
		return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to parse page token for workspace list", err)
	}

	var (
		workspaces    []slack.Team
		nextCursor    string
		ratelimitData *v2.RateLimitDescription
	)
	if o.enterpriseID != "" {
		outputAnnotations := annotations.New()
		workspaces, nextCursor, ratelimitData, err = o.enterpriseClient.GetAuthTeamsList(ctx, bag.PageToken())
		outputAnnotations.WithRateLimiting(ratelimitData)
		if err != nil {
			return nil, "", outputAnnotations, fmt.Errorf("slack-connector: failed to get auth teams list for workspace listing: %w", err)
		}
	} else {
		params := slack.ListTeamsParameters{Cursor: bag.PageToken()}
		workspaces, nextCursor, err = o.client.ListTeamsContext(ctx, params)
		if err != nil {
			annos, err := pkg.AnnotationsForError(err)
			return nil, "", annos, err
		}
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create next page token for workspace list", err)
	}

	// Seed the cache.
	for _, workspace := range workspaces {
		o.enterpriseClient.SetWorkspaceName(workspace.ID, workspace.Name)
	}

	output, err := pkg.MakeResourceList(
		ctx,
		workspaces,
		nil,
		workspaceResource,
	)
	if err != nil {
		return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to build workspace resource list", err)
	}

	return output, pageToken, nil, nil
}

func (o *workspaceResourceType) Entitlements(
	_ context.Context,
	resource *v2.Resource,
	_ *pagination.Token,
) (
	[]*v2.Entitlement,
	string,
	annotations.Annotations,
	error,
) {
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
		},
		"",
		nil,
		nil
}

func (o *workspaceResourceType) Grants(
	ctx context.Context,
	resource *v2.Resource,
	pt *pagination.Token,
) (
	[]*v2.Grant,
	string,
	annotations.Annotations,
	error,
) {
	bag, err := pkg.ParsePageToken(pt.Token, &v2.ResourceId{ResourceType: resourceTypeUser.Id})
	if err != nil {
		return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to parse page token for workspace grants", err)
	}

	outputAnnotations := annotations.New()
	users, nextCursor, ratelimitData, err := o.enterpriseClient.GetUsers(
		ctx,
		resource.Id.Resource,
		bag.PageToken(),
	)
	outputAnnotations.WithRateLimiting(ratelimitData)
	if err != nil {
		return nil, "", outputAnnotations, fmt.Errorf("slack-connector: failed to get workspace users for grants: %w", err)
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create next page token for workspace grants", err)
	}

	var rv []*v2.Grant
	for _, user := range users {
		if user.IsStranger {
			continue
		}
		userID, err := resources.NewResourceID(resourceTypeUser, user.ID)
		if err != nil {
			return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create user resource ID for workspace grant", err)
		}

		if user.IsPrimaryOwner {
			rr, err := roleResource(ctx, PrimaryOwnerRoleID, resource.Id)
			if err != nil {
				return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create primary owner role resource", err)
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsOwner {
			rr, err := roleResource(ctx, OwnerRoleID, resource.Id)
			if err != nil {
				return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create owner role resource", err)
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsAdmin {
			rr, err := roleResource(ctx, AdminRoleID, resource.Id)
			if err != nil {
				return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create admin role resource", err)
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsRestricted {
			if user.IsUltraRestricted {
				rr, err := roleResource(ctx, SingleChannelGuestRoleID, resource.Id)
				if err != nil {
					return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create single channel guest role resource", err)
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			} else {
				rr, err := roleResource(ctx, MultiChannelGuestRoleID, resource.Id)
				if err != nil {
					return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create multi channel guest role resource", err)
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			}
		}

		if user.IsInvitedUser {
			rr, err := roleResource(ctx, InvitedMemberRoleID, resource.Id)
			if err != nil {
				return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create invited member role resource", err)
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if !user.IsRestricted && !user.IsUltraRestricted && !user.IsInvitedUser && !user.IsBot && !user.Deleted {
			rr, err := roleResource(ctx, MemberRoleID, resource.Id)
			if err != nil {
				return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create member role resource", err)
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsBot {
			rr, err := roleResource(ctx, BotRoleID, resource.Id)
			if err != nil {
				return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create bot role resource", err)
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if o.enterpriseID != "" {
			if user.Enterprise.IsPrimaryOwner {
				rr, err := enterpriseRoleResource(ctx, OrganizationPrimaryOwnerID, resource.Id)
				if err != nil {
					return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create organization primary owner role resource", err)
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			}
			if user.Enterprise.IsOwner {
				rr, err := enterpriseRoleResource(ctx, OrganizationOwnerID, resource.Id)
				if err != nil {
					return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create organization owner role resource", err)
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			}
			if user.Enterprise.IsAdmin {
				rr, err := enterpriseRoleResource(ctx, OrganizationAdminID, resource.Id)
				if err != nil {
					return nil, "", nil, uhttp.WrapErrors(codes.Internal, "slack-connector: failed to create organization admin role resource", err)
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			}
		}

		// confused about Workspace vs Workspace Role? check this link:
		// https://github.com/ConductorOne/baton-slack/pull/4
		rv = append(rv, grant.NewGrant(resource, memberEntitlement, userID))
	}

	return rv, pageToken, outputAnnotations, nil
}

func (o *workspaceResourceType) Grant(
	ctx context.Context,
	principal *v2.Resource,
	entitlement *v2.Entitlement,
) (annotations.Annotations, error) {
	if o.enterpriseID == "" {
		return nil, uhttp.WrapErrors(codes.FailedPrecondition, "slack-connector: enterprise ID and enterprise token are both required")
	}

	logger := ctxzap.Extract(ctx)

	if principal.Id.ResourceType != resourceTypeUser.Id {
		logger.Warn(
			"slack-connector: only users can be assigned to a workspace",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, uhttp.WrapErrors(codes.PermissionDenied, "slack-connector: only users can be assigned to a workspace")
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
		return outputAnnotations, fmt.Errorf("slack-connector: failed to add user to workspace during grant operation: %w", err)
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
		return nil, uhttp.WrapErrors(codes.FailedPrecondition, "slack-connector: enterprise ID and enterprise token are both required to revoke grants")
	}

	logger := ctxzap.Extract(ctx)

	principal := grant.Principal
	if principal.Id.ResourceType != resourceTypeUser.Id {
		logger.Warn(
			"slack-connector: only users can be revoked from a workspace",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, uhttp.WrapErrors(codes.PermissionDenied, "slack-connector: only users can be revoked from a workspace")
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
		return outputAnnotations, fmt.Errorf("slack-connector: failed to remove user from workspace during revoke operation: %w", err)
	}

	return outputAnnotations, nil
}
