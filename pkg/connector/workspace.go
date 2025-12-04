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
	"github.com/slack-go/slack"
)

const memberEntitlement = "member"

type workspaceResourceType struct {
	resourceType       *v2.ResourceType
	client             *slack.Client
	enterpriseID       string
	businessPlusClient *enterprise.Client
}

func (o *workspaceResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func workspaceBuilder(
	client *slack.Client,
	enterpriseID string,
	businessPlusClient *enterprise.Client,
) *workspaceResourceType {
	return &workspaceResourceType{
		resourceType:       resourceTypeWorkspace,
		client:             client,
		enterpriseID:       enterpriseID,
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
		workspaces, nextCursor, ratelimitData, err = o.businessPlusClient.GetAuthTeamsList(ctx, bag.PageToken())
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

	err = o.businessPlusClient.SetWorkspaceNames(ctx, attrs.Session, workspaces)
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
	users, nextCursor, ratelimitData, err := o.businessPlusClient.GetUsers(
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

		// Only create workspace membership grants (no role-based grants)
		rv = append(rv, grant.NewGrant(resource, memberEntitlement, userID))
	}

	return rv, &resources.SyncOpResults{
		NextPageToken: pageToken,
	}, nil
}

// Grant and Revoke are not implemented for workspace membership because they require
// Enterprise Grid-only API endpoints (admin.users.assign and admin.users.remove).
// These endpoints are only available on Enterprise Grid plans, not Business+ plans.
