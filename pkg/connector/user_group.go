package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"

	enterprise "github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/slack-go/slack"
)

type userGroupResourceType struct {
	resourceType     *v2.ResourceType
	client           *slack.Client
	enterpriseID     string
	enterpriseClient *enterprise.Client
}

func (o *userGroupResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func userGroupBuilder(
	client *slack.Client,
	enterpriseID string,
	enterpriseClient *enterprise.Client,
) *userGroupResourceType {
	return &userGroupResourceType{
		resourceType:     resourceTypeUserGroup,
		client:           client,
		enterpriseID:     enterpriseID,
		enterpriseClient: enterpriseClient,
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
		userGroups    []slack.UserGroup
		ratelimitData *v2.RateLimitDescription
		err           error
	)
	outputAnnotations := annotations.New()
	// We use different method here because we need to pass a teamID, but it's
	// not supported by the slack-go library.
	if o.enterpriseID != "" {
		userGroups, ratelimitData, err = o.enterpriseClient.GetUserGroups(ctx, parentResourceID.Resource)
		outputAnnotations.WithRateLimiting(ratelimitData)
		if err != nil {
			return nil, &resource.SyncOpResults{Annotations: outputAnnotations}, err
		}
	} else {
		opts := []slack.GetUserGroupsOption{
			slack.GetUserGroupsOptionIncludeUsers(true),
			// We need to add a way to signify disabled resources in baton in
			// order to include disabled groups. We should also be doing this
			// for both enterprise and non-enterprise groups.
			// slack.GetUserGroupsOptionIncludeDisabled(true),
		}
		userGroups, err = o.client.GetUserGroupsContext(ctx, opts...)
		if err != nil {
			annos, err := AnnotationsForError(err)
			return nil, &resource.SyncOpResults{Annotations: annos}, err
		}
	}

	output, err := MakeResourceList(
		ctx,
		userGroups,
		parentResourceID,
		userGroupResource,
	)
	if err != nil {
		return nil, nil, err
	}
	return output, &resource.SyncOpResults{Annotations: outputAnnotations}, nil
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
	_ resource.SyncOpAttrs,
) (
	[]*v2.Grant,
	*resource.SyncOpResults,
	error,
) {
	outputAnnotations := annotations.New()
	// TODO(marcos): This should use 2D pagination.
	groupMembers, ratelimitData, err := o.enterpriseClient.GetUserGroupMembers(
		ctx,
		res.Id.Resource,
		res.ParentResourceId.Resource,
	)
	outputAnnotations.WithRateLimiting(ratelimitData)
	if err != nil {
		return nil, &resource.SyncOpResults{Annotations: outputAnnotations}, err
	}

	var rv []*v2.Grant
	for _, member := range groupMembers {
		user, err := o.client.GetUserInfoContext(ctx, member)
		if err != nil {
			annos, err := AnnotationsForError(err)
			return nil, &resource.SyncOpResults{Annotations: annos}, err
		}
		ur, err := userResource(ctx, user, res.Id)
		if err != nil {
			return nil, nil, err
		}

		grant := grant.NewGrant(res, memberEntitlement, ur.Id)
		rv = append(rv, grant)
	}

	return rv, &resource.SyncOpResults{}, nil
}
