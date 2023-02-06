package connector

import (
	"context"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	resource "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/slack-go/slack"
)

type userResourceType struct {
	resourceType *v2.ResourceType
	client       *slack.Client
}

func (o *userResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func (o *userResourceType) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *userResourceType) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *userResourceType) List(ctx context.Context, parentResourceID *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	users, err := o.client.GetUsersContext(ctx)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Resource
	for _, user := range users {
		ur, err := userResource(ctx, &user, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, ur)
	}

	return rv, "", nil, nil
}

func userResource(ctx context.Context, user *slack.User, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := make(map[string]interface{})
	profile["first_name"] = user.Profile.FirstName
	profile["last_name"] = user.Profile.LastName
	profile["login"] = user.Profile.Email
	profile["workspace"] = user.Profile.Team
	profile["user_id"] = user.ID

	userTraitOptions := []resource.UserTraitOption{resource.WithUserProfile(profile), resource.WithEmail(user.Profile.Email, true), resource.WithStatus(v2.UserTrait_Status_STATUS_ENABLED)}
	ret, err := resource.NewUserResource(user.Name, resourceTypeUser, user.ID, userTraitOptions, resource.WithParentResourceID(parentResourceID))
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func userBuilder(client *slack.Client) *userResourceType {
	return &userResourceType{
		resourceType: resourceTypeUser,
		client:       client,
	}
}
