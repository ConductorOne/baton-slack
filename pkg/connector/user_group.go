package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-sdk/pkg/uhttp"

	"github.com/conductorone/baton-slack/pkg"
	"github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/slack-go/slack"
	"google.golang.org/grpc/codes"
)

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
	outputAnnotations := annotations.New()
	userGroups, err = o.client.GetUserGroupsContext(ctx, slack.GetUserGroupsOptionWithTeamID(parentResourceID.Resource))
	if err != nil {
		wrappedErr := pkg.WrapSlackClientError(err, fmt.Sprintf("fetching user groups for team %s", parentResourceID.Resource))
		return nil, &resource.SyncOpResults{}, wrappedErr
	}

	rv := make([]*v2.Resource, 0, len(userGroups))
	for _, ug := range userGroups {
		resource, err := userGroupResource(ctx, ug, parentResourceID)
		if err != nil {
			return nil, nil, uhttp.WrapErrors(codes.Internal, "creating user group resource", err)
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
	_ resource.SyncOpAttrs,
) (
	[]*v2.Grant,
	*resource.SyncOpResults,
	error,
) {
	groupMembers, err := o.client.GetUserGroupMembersContext(ctx, res.Id.Resource)
	if err != nil {
		annos, err := pkg.AnnotationsForError(err)
		return nil, &resource.SyncOpResults{Annotations: annos}, err
	}

	var rv []*v2.Grant
	for _, member := range groupMembers {
		user, err := o.client.GetUserInfoContext(ctx, member)
		if err != nil {
			annos, err := pkg.AnnotationsForError(err)
			return nil, &resource.SyncOpResults{Annotations: annos}, err
		}
		ur, err := userResource(ctx, user, res.Id)
		if err != nil {
			return nil, nil, uhttp.WrapErrors(codes.Internal, "creating user resource", err)
		}

		grant := grant.NewGrant(res, memberEntitlement, ur.Id)
		rv = append(rv, grant)
	}

	return rv, &resource.SyncOpResults{}, nil
}
