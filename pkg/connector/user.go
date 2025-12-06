package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-slack/pkg"
	"github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/slack-go/slack"
)

type userResourceType struct {
	resourceType       *v2.ResourceType
	client             *slack.Client
	businessPlusClient *client.Client
}

func (o *userResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func (o *userResourceType) scimUserResource(ctx context.Context, scimUser client.UserResource, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	// NOTE: this is mainly to maintain compatibility with existing profile in non scim flow.
	slackUser, err := o.client.GetUserInfoContext(ctx, scimUser.ID)
	if err != nil {
		wrappedErr := pkg.WrapError(err, fmt.Sprintf("fetching user info for SCIM user %s", scimUser.ID))
		return nil, wrappedErr
	}

	profile := make(map[string]interface{})
	profile["first_name"] = slackUser.Profile.FirstName
	profile["last_name"] = slackUser.Profile.LastName
	profile["login"] = slackUser.Profile.Email
	profile["workspace"] = slackUser.Profile.Team
	profile["user_id"] = slackUser.ID
	profile["status_text"] = slackUser.Profile.StatusText
	profile["status_emoji"] = slackUser.Profile.StatusEmoji
	profile["is_admin"] = slackUser.IsAdmin
	profile["is_owner"] = slackUser.IsOwner
	profile["is_primary_owner"] = slackUser.IsPrimaryOwner
	profile["is_bot"] = slackUser.IsBot
	profile["is_app_user"] = slackUser.IsAppUser
	profile["is_invited_user"] = slackUser.IsInvitedUser
	profile["is_restricted"] = slackUser.IsRestricted
	profile["is_ultra_restricted"] = slackUser.IsUltraRestricted
	profile["is_stranger"] = slackUser.IsStranger
	profile["is_deleted"] = slackUser.Deleted
	profile["user_id"] = fmt.Sprint(slackUser.ID)

	userStatus := v2.UserTrait_Status_STATUS_ENABLED
	if slackUser.Deleted {
		userStatus = v2.UserTrait_Status_STATUS_DELETED
	}

	userTraitOptions := []resource.UserTraitOption{
		resource.WithUserProfile(profile),
		resource.WithEmail(slackUser.Profile.Email, true),
		resource.WithStatus(userStatus),
	}

	if slackUser.IsBot {
		userTraitOptions = append(
			userTraitOptions,
			resource.WithAccountType(v2.UserTrait_ACCOUNT_TYPE_SERVICE),
		)
	}

	// If the credentials we're hitting the API with don't have admin, this can
	// be false even if the user has mfa enabled.
	// See https://api.slack.com/types/user for more info
	if slackUser.Has2FA {
		userTraitOptions = append(
			userTraitOptions,
			resource.WithMFAStatus(&v2.UserTrait_MFAStatus{MfaEnabled: true}),
		)
	}

	return resource.NewUserResource(
		slackUser.Name,
		resourceTypeUser,
		slackUser.ID,
		userTraitOptions,
		resource.WithParentResourceID(parentResourceID),
	)
}

// Create a new connector resource for a Slack user.
func userResource(
	_ context.Context,
	user *slack.User,
	parentResourceID *v2.ResourceId,
) (*v2.Resource, error) {
	profile := make(map[string]interface{})
	profile["first_name"] = user.Profile.FirstName
	profile["last_name"] = user.Profile.LastName
	profile["login"] = user.Profile.Email
	profile["workspace"] = user.Profile.Team
	profile["user_id"] = user.ID
	profile["status_text"] = user.Profile.StatusText
	profile["status_emoji"] = user.Profile.StatusEmoji
	profile["is_admin"] = user.IsAdmin
	profile["is_owner"] = user.IsOwner
	profile["is_primary_owner"] = user.IsPrimaryOwner
	profile["is_bot"] = user.IsBot
	profile["is_app_user"] = user.IsAppUser
	profile["is_invited_user"] = user.IsInvitedUser
	profile["is_restricted"] = user.IsRestricted
	profile["is_ultra_restricted"] = user.IsUltraRestricted
	profile["is_stranger"] = user.IsStranger
	profile["is_deleted"] = user.Deleted
	profile["user_id"] = fmt.Sprint(user.ID)

	userStatus := v2.UserTrait_Status_STATUS_ENABLED
	if user.Deleted {
		userStatus = v2.UserTrait_Status_STATUS_DELETED
	}

	userTraitOptions := []resource.UserTraitOption{
		resource.WithUserProfile(profile),
		resource.WithEmail(user.Profile.Email, true),
		resource.WithStatus(userStatus),
	}

	if user.IsBot {
		userTraitOptions = append(
			userTraitOptions,
			resource.WithAccountType(v2.UserTrait_ACCOUNT_TYPE_SERVICE),
		)
	}

	// If the credentials we're hitting the API with don't have admin, this can
	// be false even if the user has mfa enabled.
	// See https://api.slack.com/types/user for more info
	if user.Has2FA {
		userTraitOptions = append(
			userTraitOptions,
			resource.WithMFAStatus(&v2.UserTrait_MFAStatus{MfaEnabled: true}),
		)
	}

	return resource.NewUserResource(
		user.Name,
		resourceTypeUser,
		user.ID,
		userTraitOptions,
		resource.WithParentResourceID(parentResourceID),
	)
}

func (o *userResourceType) Entitlements(
	_ context.Context,
	_ *v2.Resource,
	_ resource.SyncOpAttrs,
) (
	[]*v2.Entitlement,
	*resource.SyncOpResults,
	error,
) {
	return nil, &resource.SyncOpResults{}, nil
}

func (o *userResourceType) Grants(
	_ context.Context,
	_ *v2.Resource,
	_ resource.SyncOpAttrs,
) (
	[]*v2.Grant,
	*resource.SyncOpResults,
	error,
) {
	return nil, &resource.SyncOpResults{}, nil
}

func (o *userResourceType) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	attrs resource.SyncOpAttrs,
) (
	[]*v2.Resource,
	*resource.SyncOpResults,
	error,
) {
	if parentResourceID == nil {
		return nil, &resource.SyncOpResults{}, nil
	}

	l := ctxzap.Extract(ctx)
	if o.businessPlusClient != nil {
		l.Debug("Attempting to use SCIM API to list users")
		return o.listScimAPI(ctx, parentResourceID, attrs)
	}

	l.Debug("Using standard API to list users")
	return o.listStandardAPI(ctx, parentResourceID, attrs)
}

func (o *userResourceType) listStandardAPI(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	attrs resource.SyncOpAttrs,
) (
	[]*v2.Resource,
	*resource.SyncOpResults,
	error,
) {
	options := slack.GetUsersOptionTeamID(parentResourceID.Resource)
	users, err := o.client.GetUsersContext(ctx, options)
	if err != nil {
		annos, err := pkg.AnnotationsForError(err)
		return nil, &resource.SyncOpResults{Annotations: annos}, err
	}

	rv := make([]*v2.Resource, 0, len(users))
	for _, u := range users {
		resource, err := userResource(ctx, &u, parentResourceID)
		if err != nil {
			return nil, nil, pkg.WrapError(err, "creating user resource")
		}
		rv = append(rv, resource)
	}
	return rv, &resource.SyncOpResults{}, nil
}

func (o *userResourceType) listScimAPI(ctx context.Context, parentResourceID *v2.ResourceId, attrs resource.SyncOpAttrs) ([]*v2.Resource, *resource.SyncOpResults, error) {
	var err error
	startIndex := 1
	if attrs.PageToken.Token != "" {
		startIndex, err = strconv.Atoi(attrs.PageToken.Token)
		if err != nil {
			return nil, nil, pkg.WrapError(err, "parsing page token")
		}
	}

	var annos annotations.Annotations
	count := 100
	response, ratelimitData, err := o.businessPlusClient.ListIDPUsers(ctx, startIndex, count)
	annos.WithRateLimiting(ratelimitData)
	if err != nil {
		return nil, &resource.SyncOpResults{Annotations: annos}, pkg.WrapError(err, "fetching SCIM users")
	}

	rv := make([]*v2.Resource, 0, len(response.Resources))
	for _, user := range response.Resources {
		userResource, err := o.scimUserResource(ctx, user, parentResourceID)
		if err != nil {
			return nil, &resource.SyncOpResults{Annotations: annos}, err
		}
		rv = append(rv, userResource)
	}

	var nextPageToken string
	if response.TotalResults > startIndex+count-1 {
		nextPageToken = fmt.Sprint(startIndex + count)
	}
	return rv, &resource.SyncOpResults{NextPageToken: nextPageToken, Annotations: annos}, nil
}

func userBuilder(
	slackClient *slack.Client,
	businessPlusClient *client.Client,
) *userResourceType {
	return &userResourceType{
		resourceType:       resourceTypeUser,
		client:             slackClient,
		businessPlusClient: businessPlusClient,
	}
}
