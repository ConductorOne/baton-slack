package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	grant "github.com/conductorone/baton-sdk/pkg/types/grant"
	resource "github.com/conductorone/baton-sdk/pkg/types/resource"
	enterprise "github.com/conductorone/baton-slack/pkg/slack"
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

func userGroupBuilder(client *slack.Client, enterpriseID string, enterpriseClient *enterprise.Client) *userGroupResourceType {
	return &userGroupResourceType{
		resourceType:     resourceTypeUserGroup,
		client:           client,
		enterpriseID:     enterpriseID,
		enterpriseClient: enterpriseClient,
	}
}

// Create a new connector resource for a Slack user group.
func userGroupResource(ctx context.Context, userGroup slack.UserGroup, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := make(map[string]interface{})
	profile["userGroup_id"] = userGroup.ID
	profile["userGroup_name"] = userGroup.Name
	profile["userGroup_handle"] = userGroup.Handle

	groupTrait := []resource.GroupTraitOption{resource.WithGroupProfile(profile)}
	ret, err := resource.NewGroupResource(userGroup.Name, resourceTypeUserGroup, userGroup.ID, groupTrait, resource.WithParentResourceID(parentResourceID))
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (o *userGroupResourceType) List(ctx context.Context, parentResourceID *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	var userGroups []slack.UserGroup
	var err error
	// different method here because we need to pass a teamID, but it's not supported by the slack-go library
	if o.enterpriseID != "" {
		userGroups, err = o.enterpriseClient.GetUserGroups(ctx, parentResourceID.Resource)
		if err != nil {
			annos, err := annotationsForError(err)
			return nil, "", annos, err
		}
	} else {
		opts := []slack.GetUserGroupsOption{
			slack.GetUserGroupsOptionIncludeUsers(true),
			// We need to add a way to signify disabled resources in baton in order to include disabled groups
			// We should also be doing this for both enterprise and non-enterprise groups
			// slack.GetUserGroupsOptionIncludeDisabled(true),
		}
		userGroups, err = o.client.GetUserGroupsContext(ctx, opts...)
		if err != nil {
			annos, err := annotationsForError(err)
			return nil, "", annos, err
		}
	}

	rv := make([]*v2.Resource, 0, len(userGroups))
	for _, userGroup := range userGroups {
		cr, err := userGroupResource(ctx, userGroup, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, cr)
	}

	return rv, "", nil, nil
}

func (o *userGroupResourceType) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement

	assigmentOptions := []ent.EntitlementOption{
		ent.WithGrantableTo(resourceTypeUser),
		ent.WithDescription(fmt.Sprintf("Member of %s User group", resource.DisplayName)),
		ent.WithDisplayName(fmt.Sprintf("%s User group %s", resource.DisplayName, memberEntitlement)),
	}

	en := ent.NewAssignmentEntitlement(resource, memberEntitlement, assigmentOptions...)
	rv = append(rv, en)

	return rv, "", nil, nil
}

func (o *userGroupResourceType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	groupMembers, err := o.enterpriseClient.GetUserGroupMembers(ctx, resource.Id.Resource, resource.ParentResourceId.Resource)
	if err != nil {
		annos, err := annotationsForError(err)
		return nil, "", annos, err
	}

	var rv []*v2.Grant
	for _, member := range groupMembers {
		user, err := o.client.GetUserInfoContext(ctx, member)
		if err != nil {
			annos, err := annotationsForError(err)
			return nil, "", annos, err
		}
		ur, err := userResource(ctx, user, resource.Id)
		if err != nil {
			return nil, "", nil, err
		}

		grant := grant.NewGrant(resource, memberEntitlement, ur.Id)
		rv = append(rv, grant)
	}

	return rv, "", nil, nil
}
