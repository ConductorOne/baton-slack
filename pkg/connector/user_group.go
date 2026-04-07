package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"

	"github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/slack-go/slack"
)

const userGroupGrantPageSize = 20

type userGroupResourceType struct {
	resourceType       *v2.ResourceType
	client             *slack.Client
	businessPlusClient *client.Client
}

func (o *userGroupResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func userGroupBuilder(
	slackClient *slack.Client,
	businessPlusClient *client.Client,
) *userGroupResourceType {
	return &userGroupResourceType{
		resourceType:       resourceTypeUserGroup,
		client:             slackClient,
		businessPlusClient: businessPlusClient,
	}
}

// Create a new connector resource for a Slack user group.
func userGroupResource(
	ctx context.Context,
	userGroup slack.UserGroup,
	parentResourceID *v2.ResourceId,
) (*v2.Resource, error) {
	return resource.NewGroupResource(
		userGroup.Name,
		resourceTypeUserGroup,
		userGroup.ID,
		[]resource.GroupTraitOption{
			resource.WithGroupProfile(
				map[string]interface{}{
					"userGroup_id":     userGroup.ID,
					"userGroup_name":   userGroup.Name,
					"userGroup_handle": userGroup.Handle,
				},
			),
		},
		resource.WithParentResourceID(parentResourceID),
	)
}

func (o *userGroupResourceType) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	_ resource.SyncOpAttrs,
) (
	[]*v2.Resource,
	*resource.SyncOpResults,
	error,
) {
	if parentResourceID == nil {
		return nil, &resource.SyncOpResults{}, nil
	}

	var (
		userGroups []slack.UserGroup
		err        error
	)
	var outputAnnotations annotations.Annotations
	userGroups, err = o.client.GetUserGroupsContext(ctx, slack.GetUserGroupsOptionWithTeamID(parentResourceID.Resource))
	if err != nil {
		return nil, &resource.SyncOpResults{Annotations: outputAnnotations}, client.WrapError(err, fmt.Sprintf("fetching user groups for team %s", parentResourceID.Resource), &outputAnnotations)
	}

	rv := make([]*v2.Resource, 0, len(userGroups))
	for _, ug := range userGroups {
		resource, err := userGroupResource(ctx, ug, parentResourceID)
		if err != nil {
			return nil, nil, fmt.Errorf("creating user group resource: %w", err)
		}
		rv = append(rv, resource)
	}

	return rv, &resource.SyncOpResults{Annotations: outputAnnotations}, nil
}

func (o *userGroupResourceType) Entitlements(
	_ context.Context,
	res *v2.Resource,
	_ resource.SyncOpAttrs,
) (
	[]*v2.Entitlement,
	*resource.SyncOpResults,
	error,
) {
	return []*v2.Entitlement{
			entitlement.NewAssignmentEntitlement(
				res,
				memberEntitlement,
				entitlement.WithGrantableTo(resourceTypeUser),
				entitlement.WithDescription(
					fmt.Sprintf(
						"Member of %s User group",
						res.DisplayName,
					),
				),
				entitlement.WithDisplayName(
					fmt.Sprintf(
						"%s User group %s",
						res.DisplayName,
						memberEntitlement,
					),
				),
			),
		},
		&resource.SyncOpResults{},
		nil
}

func (o *userGroupResourceType) Grants(
	ctx context.Context,
	res *v2.Resource,
	attrs resource.SyncOpAttrs,
) (
	[]*v2.Grant,
	*resource.SyncOpResults,
	error,
) {
	var outputAnnotations annotations.Annotations
	groupMembers, err := o.client.GetUserGroupMembersContext(ctx, res.Id.Resource)
	if err != nil {
		return nil, &resource.SyncOpResults{Annotations: outputAnnotations}, client.WrapError(err, fmt.Sprintf("fetching user group members for group %s", res.Id.Resource), &outputAnnotations)
	}

	// Parse the page offset from the token. On the first call this is 0.
	offset := 0
	if attrs.PageToken.Token != "" {
		offset, err = strconv.Atoi(attrs.PageToken.Token)
		if err != nil {
			return nil, nil, fmt.Errorf("baton-slack: parsing page token: %w", err)
		}
	}

	// Group membership may have changed between pages.
	if offset >= len(groupMembers) {
		return nil, &resource.SyncOpResults{Annotations: outputAnnotations}, nil
	}

	// Slice the member list for this page.
	end := min(offset+userGroupGrantPageSize, len(groupMembers))
	page := groupMembers[offset:end]

	var rv []*v2.Grant
	for _, member := range page {
		user, err := o.client.GetUserInfoContext(ctx, member)
		if err != nil {
			return nil, &resource.SyncOpResults{Annotations: outputAnnotations}, client.WrapError(err, fmt.Sprintf("fetching user info for member %s", member), &outputAnnotations)
		}
		ur, err := userResource(ctx, user, res.Id)
		if err != nil {
			return nil, nil, fmt.Errorf("creating user resource: %w", err)
		}

		grant := grant.NewGrant(res, memberEntitlement, ur.Id)
		rv = append(rv, grant)
	}

	// If there are more members, return a token so the SDK calls us again.
	var nextPageToken string
	if end < len(groupMembers) {
		nextPageToken = strconv.Itoa(end)
	}

	return rv, &resource.SyncOpResults{NextPageToken: nextPageToken, Annotations: outputAnnotations}, nil
}
