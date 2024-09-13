package enterprise

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

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
) (*User, error) {
	var response struct {
		BaseResponse
		User *User `json:"user"`
	}

	_, _, err := c.post(
		ctx,
		UrlPathGetUserInfo,
		&response,
		map[string]interface{}{"user": userID},
		true,
	)
	if err := response.handleError(err, "fetching user info"); err != nil {
		return nil, err
	}

	return response.User, nil
}

// GetUserGroupMembers returns the members of the given user group from a given team.
func (c *Client) GetUserGroupMembers(
	ctx context.Context,
	userGroupID string,
	teamID string,
) ([]string, error) {
	var response struct {
		BaseResponse
		Users []string `json:"users"`
	}

	_, _, err := c.post(
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
		return nil, err
	}

	return response.Users, nil
}

// GetUsersAdmin returns all users in Enterprise grid.
func (c *Client) GetUsersAdmin(
	ctx context.Context,
	cursor string,
) ([]UserAdmin, string, error) {
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

	_, _, err := c.post(
		ctx,
		UrlPathGetUsersAdmin,
		&response,
		values,
		false,
	)
	if err := response.handleError(err, "fetching users"); err != nil {
		return nil, "", err
	}

	nextToken := response.ResponseMetadata.NextCursor
	return response.Users, nextToken, nil
}

// GetUsers returns the users of the given team.
func (c *Client) GetUsers(
	ctx context.Context,
	teamID string,
	cursor string,
) ([]User, string, error) {
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

	_, _, err := c.post(
		ctx,
		UrlPathGetUsers,
		&response,
		values,
		true,
	)
	if err := response.handleError(err, "fetching users"); err != nil {
		return nil, "", err
	}

	return response.Users, response.ResponseMetadata.NextCursor, nil
}

// GetTeams returns the teams of the given enterprise.
func (c *Client) GetTeams(
	ctx context.Context,
	cursor string,
) ([]slack.Team, string, error) {
	values := map[string]interface{}{}

	if cursor != "" {
		values["cursor"] = cursor
	}

	var response struct {
		BaseResponse
		Teams []slack.Team `json:"teams"`
		Pagination
	}

	_, _, err := c.post(
		ctx,
		UrlPathGetTeams,
		&response,
		values,
		false,
	)

	if err := response.handleError(err, "fetching teams"); err != nil {
		return nil, "", err
	}

	return response.Teams, response.ResponseMetadata.NextCursor, nil
}

// GetRoleAssignments returns the role assignments for the given role ID.
func (c *Client) GetRoleAssignments(
	ctx context.Context,
	roleID string,
	cursor string,
) ([]RoleAssignment, string, error) {
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

	_, _, err := c.post(
		ctx,
		UrlPathGetRoleAssignments,
		&response,
		values,
		false,
	)
	if err := response.handleError(err, "fetching role assignments"); err != nil {
		return nil, "", err
	}

	return response.RoleAssignments, response.ResponseMetadata.NextCursor, nil
}

// GetUserGroups returns the user groups for the given team.
func (c *Client) GetUserGroups(ctx context.Context, teamID string) ([]slack.UserGroup, error) {
	var response struct {
		BaseResponse
		UserGroups []slack.UserGroup `json:"usergroups"`
	}

	_, _, err := c.post(
		ctx,
		UrlPathGetUserGroups,
		&response,
		map[string]interface{}{"team_id": teamID},
		// bot token needed here cause user token doesn't work unless user is in all workspaces
		true,
	)
	if err := response.handleError(err, "fetching user groups"); err != nil {
		return nil, err
	}

	return response.UserGroups, nil
}

// SetWorkspaceRole sets the role for the given user in the given team.
func (c *Client) SetWorkspaceRole(
	ctx context.Context,
	teamID string,
	userID string,
	roleID string,
) error {
	actionUrl, err := getWorkspaceUrlPathByRole(roleID)
	if err != nil {
		return err
	}

	var response BaseResponse

	_, _, err = c.post(
		ctx,
		actionUrl,
		&response,
		map[string]interface{}{
			"team_id": teamID,
			"user_id": userID,
		},
		false,
	)
	return response.handleError(err, "setting user role")
}

// ListIDPGroups returns all IDP groups from the SCIM API.
func (c *Client) ListIDPGroups(ctx context.Context) ([]GroupResource, error) {
	var allGroups []GroupResource
	startIndex := 1

	for {
		var response SCIMResponse[GroupResource]
		_, _, err := c.getScim(
			ctx,
			UrlPathIDPGroups,
			&response,
			map[string]interface{}{
				"startIndex": startIndex,
				"count":      PageSizeDefault,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("error fetching IDP groups: %w", err)
		}

		allGroups = append(allGroups, response.Resources...)

		startIndex += response.ItemsPerPage

		if response.TotalResults < startIndex {
			break
		}
	}

	return allGroups, nil
}

// GetIDPGroup returns a single IDP group from the SCIM API.
func (c *Client) GetIDPGroup(
	ctx context.Context,
	groupID string,
) (*GroupResource, error) {
	var response GroupResource
	_, _, err := c.getScim(
		ctx,
		fmt.Sprintf(UrlPathIDPGroup, groupID),
		&response,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("error fetching IDP group: %w", err)
	}

	return &response, nil
}

// AddUserToGroup patches a group by adding a user to it.
func (c *Client) AddUserToGroup(
	ctx context.Context,
	groupID string,
	user string,
) error {
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

	err := c.patchGroup(ctx, groupID, requestBody)
	if err != nil {
		return fmt.Errorf("error adding user to IDP group: %w", err)
	}

	return nil
}

// RemoveUserFromGroup patches a group by removing a user from it.
func (c *Client) RemoveUserFromGroup(
	ctx context.Context,
	groupID string,
	user string,
) error {
	// need to fetch group to get existing members
	group, err := c.GetIDPGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("error fetching IDP group: %w", err)
	}

	var result []UserID
	for _, member := range group.Members {
		if member.Value != user {
			result = append(result, UserID{Value: member.Value})
		}
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

	err = c.patchGroup(ctx, groupID, requestBody)
	if err != nil {
		return fmt.Errorf("error removing user from IDP group: %w", err)
	}

	return nil
}

func (c *Client) patchGroup(
	ctx context.Context,
	groupID string,
	requestBody PatchOp,
) error {
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	var response *GroupResource
	_, _, err = c.patchScim(
		ctx,
		fmt.Sprintf(UrlPathIDPGroup, groupID),
		&response,
		payload,
	)
	if err != nil {
		return fmt.Errorf("error patching IDP group: %w", err)
	}

	return nil
}
