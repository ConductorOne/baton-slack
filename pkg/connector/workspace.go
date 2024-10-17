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
	"github.com/conductorone/baton-slack/pkg"
	enterprise "github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/slack-go/slack"
)

var workspacesNameCache = make(map[string]string)

const memberEntitlement = "member"

type workspaceResourceType struct {
	resourceType     *v2.ResourceType
	client           *slack.Client
	enterpriseID     string
	enterpriseClient *enterprise.Client
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
		resourceType:     resourceTypeWorkspace,
		client:           client,
		enterpriseID:     enterpriseID,
		enterpriseClient: enterpriseClient,
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
		return nil, "", nil, err
	}

	var (
		workspaces    []slack.Team
		nextCursor    string
		ratelimitData *v2.RateLimitDescription
	)
	if o.enterpriseID != "" {
		outputAnnotations := annotations.New()
		workspaces, nextCursor, ratelimitData, err = o.enterpriseClient.GetTeams(ctx, bag.PageToken())
		outputAnnotations.WithRateLimiting(ratelimitData)
		if err != nil {
			return nil, "", outputAnnotations, err
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
		return nil, "", nil, err
	}

	// Seed the cache.
	for _, workspace := range workspaces {
		workspacesNameCache[workspace.ID] = workspace.Name
	}

	output, err := pkg.MakeResourceList(
		ctx,
		workspaces,
		nil,
		workspaceResource,
	)
	if err != nil {
		return nil, "", nil, err
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
		return nil, "", nil, err
	}

	outputAnnotations := annotations.New()
	users, nextCursor, ratelimitData, err := o.enterpriseClient.GetUsers(
		ctx,
		resource.Id.Resource,
		bag.PageToken(),
	)
	outputAnnotations.WithRateLimiting(ratelimitData)
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Grant
	for _, user := range users {
		if user.IsStranger {
			continue
		}
		userID, err := resources.NewResourceID(resourceTypeUser, user.ID)
		if err != nil {
			return nil, "", nil, err
		}

		if user.IsPrimaryOwner {
			rr, err := roleResource(ctx, PrimaryOwnerRoleID, resource.Id)
			if err != nil {
				return nil, "", nil, err
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsOwner {
			rr, err := roleResource(ctx, OwnerRoleID, resource.Id)
			if err != nil {
				return nil, "", nil, err
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsAdmin {
			rr, err := roleResource(ctx, AdminRoleID, resource.Id)
			if err != nil {
				return nil, "", nil, err
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsRestricted {
			if user.IsUltraRestricted {
				rr, err := roleResource(ctx, SingleChannelGuestRoleID, resource.Id)
				if err != nil {
					return nil, "", nil, err
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			} else {
				rr, err := roleResource(ctx, MultiChannelGuestRoleID, resource.Id)
				if err != nil {
					return nil, "", nil, err
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			}
		}

		if user.IsInvitedUser {
			rr, err := roleResource(ctx, InvitedMemberRoleID, resource.Id)
			if err != nil {
				return nil, "", nil, err
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if !user.IsRestricted && !user.IsUltraRestricted && !user.IsInvitedUser && !user.IsBot && !user.Deleted {
			rr, err := roleResource(ctx, MemberRoleID, resource.Id)
			if err != nil {
				return nil, "", nil, err
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if user.IsBot {
			rr, err := roleResource(ctx, BotRoleID, resource.Id)
			if err != nil {
				return nil, "", nil, err
			}
			rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
		}

		if o.enterpriseID != "" {
			if user.Enterprise.IsPrimaryOwner {
				rr, err := enterpriseRoleResource(ctx, OrganizationPrimaryOwnerID, resource.Id)
				if err != nil {
					return nil, "", nil, err
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			}
			if user.Enterprise.IsOwner {
				rr, err := enterpriseRoleResource(ctx, OrganizationOwnerID, resource.Id)
				if err != nil {
					return nil, "", nil, err
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			}
			if user.Enterprise.IsAdmin {
				rr, err := enterpriseRoleResource(ctx, OrganizationAdminID, resource.Id)
				if err != nil {
					return nil, "", nil, err
				}
				rv = append(rv, grant.NewGrant(rr, RoleAssignmentEntitlement, userID))
			}
		}

		rv = append(rv, grant.NewGrant(resource, memberEntitlement, userID))
	}

	return rv, pageToken, outputAnnotations, nil
}
