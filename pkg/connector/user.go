package connector

import (
	"context"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/helpers"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	enterprise "github.com/conductorone/baton-slack/pkg/slack"
	"github.com/slack-go/slack"
)

type userResourceType struct {
	resourceType     *v2.ResourceType
	client           *slack.Client
	enterpriseID     string
	enterpriseClient *enterprise.Client
}

func (o *userResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

// Create a new connector resource for a Slack user.
func userResource(ctx context.Context, user *slack.User, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := make(map[string]interface{})
	profile["first_name"] = user.Profile.FirstName
	profile["last_name"] = user.Profile.LastName
	profile["login"] = user.Profile.Email
	profile["workspace"] = user.Profile.Team
	profile["user_id"] = user.ID
	profile["status_text"] = user.Profile.StatusText
	profile["status_emoji"] = user.Profile.StatusEmoji

	var userStatus v2.UserTrait_Status_Status
	if user.Deleted {
		userStatus = v2.UserTrait_Status_STATUS_DELETED
	} else {
		userStatus = v2.UserTrait_Status_STATUS_ENABLED
	}

	userTraitOptions := []resource.UserTraitOption{resource.WithUserProfile(profile), resource.WithEmail(user.Profile.Email, true), resource.WithStatus(userStatus)}
	ret, err := resource.NewUserResource(user.Name, resourceTypeUser, user.ID, userTraitOptions, resource.WithParentResourceID(parentResourceID))
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// Create a new connector resource for a base Slack user.
// Admin API doesn't return the same values as the user API.
// We need to create a base resource for users without workspace that are fetched by the Admin API.
func baseUserResource(ctx context.Context, user enterprise.UserAdmin) (*v2.Resource, error) {
	firstname, lastname := helpers.SplitFullName(user.FullName)
	profile := make(map[string]interface{})
	profile["first_name"] = firstname
	profile["last_name"] = lastname
	profile["login"] = user.Email
	profile["user_id"] = user.ID
	profile["sso_user"] = user.HasSso

	var userStatus v2.UserTrait_Status_Status
	if user.IsActive {
		userStatus = v2.UserTrait_Status_STATUS_ENABLED
	} else {
		userStatus = v2.UserTrait_Status_STATUS_DISABLED
	}

	userTraitOptions := []resource.UserTraitOption{resource.WithUserProfile(profile), resource.WithEmail(user.Email, true), resource.WithStatus(userStatus)}
	ret, err := resource.NewUserResource(user.FullName, resourceTypeUser, user.ID, userTraitOptions)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (o *userResourceType) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *userResourceType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *userResourceType) List(ctx context.Context, parentResourceID *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	var allUsers []enterprise.UserAdmin
	var pageToken string
	var nextCursor string

	if o.enterpriseID != "" {
		bag, err := parsePageToken(pt.Token, &v2.ResourceId{ResourceType: resourceTypeUser.Id})
		if err != nil {
			return nil, "", nil, err
		}
		// need to fetch all users because users without workspace won't be fetched by GetUsersContext
		allUsers, nextCursor, err = o.enterpriseClient.GetUsersAdmin(ctx, bag.PageToken())
		if err != nil {
			annos, err := annotationsForError(err)
			return nil, "", annos, err
		}
		pageToken, err = bag.NextToken(nextCursor)
		if err != nil {
			return nil, "", nil, err
		}
	}

	users, err := o.client.GetUsersContext(ctx, slack.GetUsersOptionTeamID(parentResourceID.Resource))
	if err != nil {
		annos, err := annotationsForError(err)
		return nil, "", annos, err
	}

	var rv []*v2.Resource

	// create a base resource if user has no workspace
	for _, user := range allUsers {
		if len(user.Workspaces) == 0 {
			ur, err := baseUserResource(ctx, user)
			if err != nil {
				return nil, "", nil, err
			}
			rv = append(rv, ur)
		}
	}

	// users without workspace won't be part of users array
	for _, user := range users {
		userCopy := user
		ur, err := userResource(ctx, &userCopy, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, ur)
	}

	return rv, pageToken, nil, nil
}

func userBuilder(client *slack.Client, enterpriseID string, enterpriseClient *enterprise.Client) *userResourceType {
	return &userResourceType{
		resourceType:     resourceTypeUser,
		client:           client,
		enterpriseID:     enterpriseID,
		enterpriseClient: enterpriseClient,
	}
}
