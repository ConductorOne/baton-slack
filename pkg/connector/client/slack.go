package enterprise

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/slack-go/slack"
)

const (
	PageSizeDefault = 100
)

type Client struct {
	baseScimUrl  *url.URL
	baseUrl      *url.URL
	token        string
	enterpriseID string
	botToken     string
	ssoEnabled   bool
	wrapper      *uhttp.BaseHttpClient
}

func NewClient(
	httpClient *http.Client,
	token string,
	botToken string,
	enterpriseID string,
	ssoEnabled bool,
) (*Client, error) {
	baseUrl0, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}

	baseScimUrl0, err := url.Parse(baseScimUrl)
	if err != nil {
		return nil, err
	}

	return &Client{
		baseUrl:      baseUrl0,
		baseScimUrl:  baseScimUrl0,
		token:        token,
		enterpriseID: enterpriseID,
		botToken:     botToken,
		ssoEnabled:   ssoEnabled,
		wrapper:      uhttp.NewBaseHttpClient(httpClient),
	}, nil
}

// handleError - Slack can return a 200 with an error in the JSON body.
// Generally, it is bad practice to use interpolation in error message
// construction. It makes it difficult to find the failing code when debugging.
func (a BaseResponse) handleError(err error, action string) error {
	if err != nil {
		return fmt.Errorf("baton-slack: error %s: %w", action, err)
	}

	if a.Error != "" {
		return fmt.Errorf(
			"baton-slack: error %s: error %v needed %v provided %v",
			action,
			a.Error,
			a.Needed,
			a.Provided,
		)
	}
	return nil
}

// GetUserInfo returns the user info for the given user ID.
func (c *Client) GetUserInfo(
	ctx context.Context,
	userID string,
) (
	*User,
	*v2.RateLimitDescription,
	error,
) {
	var response struct {
		BaseResponse
		User *User `json:"user"`
	}

	ratelimitData, err := c.post(
		ctx,
		UrlPathGetUserInfo,
		&response,
		map[string]interface{}{"user": userID},
		true,
	)
	if err := response.handleError(err, "fetching user info"); err != nil {
		return nil, ratelimitData, err
	}

	return response.User, ratelimitData, nil
}

// GetUserGroupMembers returns the members of the given user group from a given team.
func (c *Client) GetUserGroupMembers(
	ctx context.Context,
	userGroupID string,
	teamID string,
) (
	[]string,
	*v2.RateLimitDescription,
	error,
) {
	var response struct {
		BaseResponse
		Users []string `json:"users"`
	}

	ratelimitData, err := c.post(
		ctx,
		UrlPathGetUserGroupMembers,
		&response,
		map[string]interface{}{
			"team_id":   teamID,
			"usergroup": userGroupID,
		},
		true,
	)
	if err := response.handleError(err, "fetching user group members"); err != nil {
		return nil, ratelimitData, err
	}

	return response.Users, ratelimitData, nil
}

// GetUsersAdmin returns all users in Enterprise grid.
func (c *Client) GetUsersAdmin(
	ctx context.Context,
	cursor string,
) (
	[]UserAdmin,
	string,
	*v2.RateLimitDescription,
	error,
) {
	values := map[string]interface{}{}

	// We need to check if cursor is empty because API throws error if empty string is passed.
	if cursor != "" {
		values["cursor"] = cursor
	}

	var response struct {
		BaseResponse
		Users []UserAdmin `json:"users"`
		Pagination
	}

	ratelimitData, err := c.post(
		ctx,
		UrlPathGetUsersAdmin,
		&response,
		values,
		false,
	)
	if err := response.handleError(err, "fetching users"); err != nil {
		return nil, "", ratelimitData, err
	}

	nextToken := response.ResponseMetadata.NextCursor
	return response.Users, nextToken, ratelimitData, nil
}

// GetUsers returns the users of the given team.
func (c *Client) GetUsers(
	ctx context.Context,
	teamID string,
	cursor string,
) (
	[]User,
	string,
	*v2.RateLimitDescription,
	error,
) {
	values := map[string]interface{}{"team_id": teamID}

	// need to check if cursor is empty because API throws error if empty string is passed
	if cursor != "" {
		values["cursor"] = cursor
	}

	var response struct {
		BaseResponse
		Users []User `json:"members"`
		Pagination
	}

	ratelimitData, err := c.post(
		ctx,
		UrlPathGetUsers,
		&response,
		values,
		true,
	)
	if err := response.handleError(err, "fetching users"); err != nil {
		return nil, "", ratelimitData, err
	}

	return response.Users,
		response.ResponseMetadata.NextCursor,
		ratelimitData,
		nil
}

// GetTeams returns the teams of the given enterprise.
func (c *Client) GetTeams(
	ctx context.Context,
	cursor string,
) (
	[]slack.Team,
	string,
	*v2.RateLimitDescription,
	error,
) {
	values := map[string]interface{}{}

	if cursor != "" {
		values["cursor"] = cursor
	}

	var response struct {
		BaseResponse
		Teams []slack.Team `json:"teams"`
		Pagination
	}

	ratelimitData, err := c.post(
		ctx,
		UrlPathGetTeams,
		&response,
		values,
		false,
	)

	if err := response.handleError(err, "fetching teams"); err != nil {
		return nil, "", ratelimitData, err
	}

	return response.Teams,
		response.ResponseMetadata.NextCursor,
		ratelimitData,
		nil
}

// GetRoleAssignments returns the role assignments for the given role ID.
func (c *Client) GetRoleAssignments(
	ctx context.Context,
	roleID string,
	cursor string,
) (
	[]RoleAssignment,
	string,
	*v2.RateLimitDescription,
	error,
) {
	values := map[string]interface{}{}

	if roleID != "" {
		values["role_ids"] = roleID
	}

	if cursor != "" {
		values["cursor"] = cursor
	}

	var response struct {
		BaseResponse
		RoleAssignments []RoleAssignment `json:"role_assignments"`
		Pagination
	}

	ratelimitData, err := c.post(
		ctx,
		UrlPathGetRoleAssignments,
		&response,
		values,
		false,
	)
	if err := response.handleError(err, "fetching role assignments"); err != nil {
		return nil, "", ratelimitData, err
	}

	return response.RoleAssignments,
		response.ResponseMetadata.NextCursor,
		ratelimitData,
		nil
}

// GetUserGroups returns the user groups for the given team.
func (c *Client) GetUserGroups(
	ctx context.Context,
	teamID string,
) (
	[]slack.UserGroup,
	*v2.RateLimitDescription,
	error,
) {
	var response struct {
		BaseResponse
		UserGroups []slack.UserGroup `json:"usergroups"`
	}

	ratelimitData, err := c.post(
		ctx,
		UrlPathGetUserGroups,
		&response,
		map[string]interface{}{"team_id": teamID},
		// The bot token needed here because user token doesn't work unless user
		// is in all workspaces.
		true,
	)
	if err := response.handleError(err, "fetching user groups"); err != nil {
		return nil, ratelimitData, err
	}

	return response.UserGroups, ratelimitData, nil
}

// GetAuthTeamsList returns the list of teams for which the app is authed.
func (c *Client) GetAuthTeamsList(
	ctx context.Context,
	cursor string,
) (
	[]slack.Team,
	string,
	*v2.RateLimitDescription,
	error,
) {
	values := map[string]interface{}{}

	if cursor != "" {
		values["cursor"] = cursor
	}

	var response struct {
		BaseResponse
		Teams []slack.Team `json:"teams"`
		Pagination
	}

	ratelimitData, err := c.post(
		ctx,
		UrlPathAuthTeamsList,
		&response,
		values,
		false,
	)
	if err := response.handleError(err, "fetching authed teams"); err != nil {
		return nil, "", ratelimitData, err
	}

	return response.Teams,
		response.ResponseMetadata.NextCursor,
		ratelimitData,
		nil
}

// SetWorkspaceRole sets the role for the given user in the given team.
func (c *Client) SetWorkspaceRole(
	ctx context.Context,
	teamID string,
	userID string,
	roleID string,
) (
	*v2.RateLimitDescription,
	error,
) {
	actionUrl, err := getWorkspaceUrlPathByRole(roleID)
	if err != nil {
		return nil, err
	}

	var response BaseResponse

	ratelimitData, err := c.post(
		ctx,
		actionUrl,
		&response,
		map[string]interface{}{
			"team_id": teamID,
			"user_id": userID,
		},
		false,
	)
	return ratelimitData, response.handleError(err, "setting user role")
}

// ListIDPGroups returns all IDP groups from the SCIM API.
func (c *Client) ListIDPGroups(
	ctx context.Context,
	startIndex int,
	count int,
) (
	*SCIMResponse[GroupResource],
	*v2.RateLimitDescription,
	error,
) {
	var response SCIMResponse[GroupResource]
	ratelimitData, err := c.getScim(
		ctx,
		UrlPathIDPGroups,
		&response,
		map[string]interface{}{
			"startIndex": startIndex,
			"count":      count,
		},
	)
	if err != nil {
		return nil, ratelimitData, fmt.Errorf("error fetching IDP groups: %w", err)
	}

	return &response, ratelimitData, nil
}

// GetIDPGroup returns a single IDP group from the SCIM API.
func (c *Client) GetIDPGroup(
	ctx context.Context,
	groupID string,
) (
	*GroupResource,
	*v2.RateLimitDescription,
	error,
) {
	var response GroupResource
	ratelimitData, err := c.getScim(
		ctx,
		fmt.Sprintf(UrlPathIDPGroup, groupID),
		&response,
		nil,
	)
	if err != nil {
		return nil, ratelimitData, fmt.Errorf("error fetching IDP group: %w", err)
	}

	return &response, ratelimitData, nil
}

// AddUserToGroup patches a group by adding a user to it.
func (c *Client) AddUserToGroup(
	ctx context.Context,
	groupID string,
	user string,
) (
	*v2.RateLimitDescription,
	error,
) {
	requestBody := PatchOp{
		Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		Operations: []ScimOperate{
			{
				Op:   "add",
				Path: "members",
				Value: []UserID{
					{Value: user},
				},
			},
		},
	}

	ratelimitData, err := c.patchGroup(ctx, groupID, requestBody)
	if err != nil {
		return ratelimitData, fmt.Errorf("error adding user to IDP group: %w", err)
	}

	return ratelimitData, nil
}

// RemoveUserFromGroup patches a group by removing a user from it.
func (c *Client) RemoveUserFromGroup(
	ctx context.Context,
	groupID string,
	user string,
) (
	bool,
	*v2.RateLimitDescription,
	error,
) {
	// First, we need to fetch group to get existing members.
	group, ratelimitData, err := c.GetIDPGroup(ctx, groupID)
	if err != nil {
		return false, ratelimitData, fmt.Errorf("error fetching IDP group: %w", err)
	}

	found := false
	var result []UserID
	for _, member := range group.Members {
		if member.Value == user {
			found = true
		} else {
			result = append(result, UserID{Value: member.Value})
		}
	}

	// If we don't find the user, we can short-circuit here.
	if !found {
		return false, ratelimitData, nil
	}

	requestBody := PatchOp{
		Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		Operations: []ScimOperate{
			{
				Op:    "replace",
				Path:  "members",
				Value: result,
			},
		},
	}

	ratelimitData, err = c.patchGroup(ctx, groupID, requestBody)
	if err != nil {
		return false, ratelimitData, fmt.Errorf("error removing user from IDP group: %w", err)
	}

	return true, ratelimitData, nil
}

func (c *Client) patchGroup(
	ctx context.Context,
	groupID string,
	requestBody PatchOp,
) (
	*v2.RateLimitDescription,
	error,
) {
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	var response *GroupResource
	ratelimitData, err := c.patchScim(
		ctx,
		fmt.Sprintf(UrlPathIDPGroup, groupID),
		&response,
		payload,
	)
	if err != nil {
		return ratelimitData, fmt.Errorf("error patching IDP group: %w", err)
	}

	return ratelimitData, nil
}
