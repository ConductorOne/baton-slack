package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-slack/pkg"
	enterprise "github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/slack-go/slack"
)

type userResourceType struct {
	resourceType     *v2.ResourceType
	client           *slack.Client
	enterpriseID     string
	enterpriseClient *enterprise.Client
	ssoEnabled       bool
}

func (o *userResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

// scimUserResource creates a user resource using the SCIM user data.
func (o *userResourceType) scimUserResource(ctx context.Context, user enterprise.UserResource) (*v2.Resource, error) {
	// Get primary email
	var primaryEmail string
	for _, email := range user.Emails {
		if email.Primary {
			primaryEmail = email.Value
			break
		}
	}
	if primaryEmail == "" && len(user.Emails) > 0 {
		primaryEmail = user.Emails[0].Value
	}

	displayName := user.DisplayName
	if displayName == "" {
		displayName = user.UserName
	}

	profile := make(map[string]interface{})
	profile["first_name"] = user.Name.GivenName
	profile["last_name"] = user.Name.FamilyName
	profile["display_name"] = user.DisplayName
	profile["login"] = primaryEmail
	profile["user_id"] = user.ID
	profile["user_name"] = user.UserName

	var userStatus v2.UserTrait_Status_Status
	if user.Active {
		userStatus = v2.UserTrait_Status_STATUS_ENABLED
	} else {
		userStatus = v2.UserTrait_Status_STATUS_DISABLED
	}

	userTraitOptions := []resource.UserTraitOption{
		resource.WithUserProfile(profile),
		resource.WithStatus(userStatus),
		resource.WithUserLogin(user.UserName),
	}

	if primaryEmail != "" {
		userTraitOptions = append(
			userTraitOptions,
			resource.WithEmail(primaryEmail, true),
		)
	}


	return resource.NewUserResource(
		displayName,
		resourceTypeUser,
		user.ID,
		userTraitOptions,
	)
}

// Create a new user from the standard Slack API user data.
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

func (o *userResourceType) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	attrs resource.SyncOpAttrs,
) (
	[]*v2.Resource,
	*resource.SyncOpResults,
	error,
) {
	// if we have an enterprise client, use the SCIM API to list users (lists all users in the enterprise)
	// including those without a workspace
	if o.enterpriseClient != nil && o.ssoEnabled {
		return o.listScimAPI(ctx, parentResourceID, attrs)
	}
	// standard Slack API to list users in the given workspace
	return o.listUsers(ctx, parentResourceID, attrs)
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

// listScimAPI lists users using the SCIM API.
// requires enterprise client and SSO to be enabled.
func (o *userResourceType) listScimAPI(ctx context.Context, parentResourceID *v2.ResourceId, attrs resource.SyncOpAttrs) ([]*v2.Resource, *resource.SyncOpResults, error) {
	if o.enterpriseClient == nil {
		return nil, nil, fmt.Errorf("baton-slack: SCIM API requires enterprise client")
	}
	l := ctxzap.Extract(ctx)
	l.Info("Listing Slack users using SCIM API")

	var err error
	startIndex := 1
	if attrs.PageToken.Token != "" {
		startIndex, err = strconv.Atoi(attrs.PageToken.Token)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid page token: %w", err)
		}
	}

	var annos annotations.Annotations
	count := 100
	response, ratelimitData, err := o.enterpriseClient.ListIDPUsers(ctx, startIndex, count)
	annos.WithRateLimiting(ratelimitData)
	if err != nil {
		return nil, &resource.SyncOpResults{Annotations: annos}, fmt.Errorf("error fetching SCIM users: %w", err)
	}

	rv := make([]*v2.Resource, 0, len(response.Resources))
	for _, user := range response.Resources {
		userResource, err := o.scimUserResource(ctx, user)
		if err != nil {
			return nil, &resource.SyncOpResults{Annotations: annos}, fmt.Errorf("error creating user resource: %w", err)
		}
		rv = append(rv, userResource)
	}

	var nextPageToken string
	if response.TotalResults > startIndex+count-1 {
		nextPageToken = fmt.Sprint(startIndex + count)
	}
	return rv, &resource.SyncOpResults{NextPageToken: nextPageToken, Annotations: annos}, nil
}

// listUsers lists users using the standard Slack API.
// does not require enterprise client.
func (o *userResourceType) listUsers(ctx context.Context, parentResourceID *v2.ResourceId, attrs resource.SyncOpAttrs) ([]*v2.Resource, *resource.SyncOpResults, error) {
	if parentResourceID == nil {
		return nil, &resource.SyncOpResults{}, nil
	}

	l := ctxzap.Extract(ctx)
	l.Info("Listing Slack users using standard API")

	var (
		allUsers      []enterprise.UserAdmin
		pageToken     string
		nextCursor    string
		ratelimitData *v2.RateLimitDescription
	)
	outputAnnotations := annotations.New()
	if o.enterpriseID != "" {
		bag, err := pkg.ParsePageToken(attrs.PageToken.Token, &v2.ResourceId{ResourceType: resourceTypeUser.Id})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse page token: %w", err)
		}

		// We need to fetch all users because users without workspace won't be
		// fetched by GetUsersContext.
		allUsers, nextCursor, ratelimitData, err = o.enterpriseClient.GetUsersAdmin(ctx, bag.PageToken())
		outputAnnotations.WithRateLimiting(ratelimitData)
		if err != nil {
			return nil, &resource.SyncOpResults{Annotations: outputAnnotations}, err
		}
		pageToken, err = bag.NextToken(nextCursor)
		if err != nil {
			return nil, nil, err
		}
	}

	options := slack.GetUsersOptionTeamID(parentResourceID.Resource)
	users, err := o.client.GetUsersContext(ctx, options)
	if err != nil {
		annos, err := pkg.AnnotationsForError(err)
		return nil, &resource.SyncOpResults{Annotations: annos}, err
	}

	// Create a base resource if user has no workspace.
	rv0, err := pkg.MakeResourceList(
		ctx,
		allUsers,
		nil,
		baseUserResource,
	)
	if err != nil {
		return nil, nil, err
	}

	// Users without workspace won't be part of users array.
	rv1, err := pkg.MakeResourceList(
		ctx,
		users,
		parentResourceID,
		func(
			ctx context.Context,
			object slack.User,
			parentResourceID *v2.ResourceId,
		) (
			*v2.Resource,
			error,
		) {
			return userResource(ctx, &object, parentResourceID)
		},
	)
	if err != nil {
		return nil, nil, err
	}
	return append(rv0, rv1...), &resource.SyncOpResults{NextPageToken: pageToken, Annotations: outputAnnotations}, nil
}

func (o *userResourceType) CreateAccount(
	ctx context.Context,
	accountInfo *v2.AccountInfo,
	credentialOptions *v2.LocalCredentialOptions,
) (
	connectorbuilder.CreateAccountResponse,
	[]*v2.PlaintextData,
	annotations.Annotations,
	error,
) {
	params, err := getInviteUserParams(accountInfo)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("baton-slack: create account get InviteUserParams failed %w", err)
	}

	if o.enterpriseClient == nil {
		return nil, nil, nil, fmt.Errorf("baton-slack: account provisioning only works for slack enterprise: %w", err)
	}

	ratelimitData, err := o.enterpriseClient.InviteUserToWorkspace(ctx, params)
	if err != nil {
		return nil, nil, nil, err
	}

	user, err := o.client.GetUserByEmail(params.Email)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("baton-slack: get user by email failed: %w", err)
	}

	outputAnnotations := annotations.New()
	outputAnnotations.WithRateLimiting(ratelimitData)

	parentResourceID, err := resource.NewResourceID(resourceTypeWorkspace, params.TeamID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("baton-slack: create parent resource failed: %w", err)
	}

	r, err := userResource(ctx, user, parentResourceID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("baton-slack: cannot create user resource: %w", err)
	}

	return &v2.CreateAccountResponse_SuccessResult{
		Resource: r,
	}, nil, outputAnnotations, nil
}

func (o *userResourceType) CreateAccountCapabilityDetails(ctx context.Context) (*v2.CredentialDetailsAccountProvisioning, annotations.Annotations, error) {
	return &v2.CredentialDetailsAccountProvisioning{
		SupportedCredentialOptions: []v2.CapabilityDetailCredentialOption{
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
		},
		PreferredCredentialOption: v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
	}, nil, nil
}

func getInviteUserParams(accountInfo *v2.AccountInfo) (*enterprise.InviteUserParams, error) {
	pMap := accountInfo.Profile.AsMap()
	email, ok := pMap["email"].(string)
	if !ok || email == "" {
		return nil, fmt.Errorf("email is required")
	}

	chanIDs, ok := pMap["channel_ids"].(string)
	if !ok || chanIDs == "" {
		return nil, fmt.Errorf("channal_ids is required")
	}

	teamID, ok := pMap["team_id"].(string)
	if !ok || teamID == "" {
		return nil, fmt.Errorf("team_id is required")
	}
	return &enterprise.InviteUserParams{
		TeamID:     teamID,
		ChannelIDs: chanIDs,
		Email:      email,
	}, nil
}


// baseUserResource creates a base user resource without workspace info.
func baseUserResource(
	_ context.Context,
	user enterprise.UserAdmin,
	_ *v2.ResourceId,
) (*v2.Resource, error) {
	profile := make(map[string]interface{})
	profile["user_id"] = user.ID
	profile["sso_user"] = user.HasSso

	userStatus := v2.UserTrait_Status_STATUS_ENABLED
	if !user.IsActive {
		userStatus = v2.UserTrait_Status_STATUS_DISABLED
	}

	userTraitOptions := []resource.UserTraitOption{
		resource.WithUserProfile(profile),
		resource.WithStatus(userStatus),
	}

	if user.HasSso {
		ssoStatus := &v2.UserTrait_SSOStatus{SsoEnabled: user.HasSso}
		userTraitOptions = append(userTraitOptions, resource.WithSSOStatus(ssoStatus))
	}

	if user.Has2Fa {
		userTraitOptions = append(
			userTraitOptions,
			resource.WithMFAStatus(&v2.UserTrait_MFAStatus{MfaEnabled: true}),
		)
	}

	if user.IsBot {
		userTraitOptions = append(
			userTraitOptions,
			resource.WithAccountType(v2.UserTrait_ACCOUNT_TYPE_SERVICE),
		)
	}

	displayName := user.ID
	if user.Email != "" {
		userTraitOptions = append(
			userTraitOptions,
			resource.WithEmail(user.Email, true),
		)
		displayName = user.Email
	}

	return resource.NewUserResource(
		displayName,
		resourceTypeUser,
		user.ID,
		userTraitOptions,
	)
}

func userBuilder(
	client *slack.Client,
	enterpriseID string,
	enterpriseClient *enterprise.Client,
) *userResourceType {
	return &userResourceType{
		resourceType:     resourceTypeUser,
		client:           client,
		enterpriseID:     enterpriseID,
		enterpriseClient: enterpriseClient,
		ssoEnabled:       true, // Enable SSO features for SCIM API access
	}
}

