package enterprise

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/slack-go/slack"
)

const (
	PageSizeDefault = 100

	// Slack API error string constants.
	SlackErrUserAlreadyTeamMember = "user_already_team_member"
	SlackErrUserAlreadyDeleted    = "user_already_deleted"
	ScimVersionV2                 = "v2"
	ScimVersionV1                 = "v1"
)

type Client struct {
	baseScimUrl              *url.URL
	baseUrl                  *url.URL
	token                    string
	enterpriseID             string
	botToken                 string
	ssoEnabled               bool
	scimVersion              string
	wrapper                  *uhttp.BaseHttpClient
	workspacesNameCache      map[string]string
	workspacesNameCacheMutex sync.RWMutex
}

func NewClient(
	httpClient *http.Client,
	token string,
	botToken string,
	enterpriseID string,
	ssoEnabled bool,
	govEnv bool,
) (*Client, error) {
	finalBaseUrl := baseUrl
	finalBaseScimUrl := baseScimUrl
	finalScimVersion := ScimVersionV2
	if govEnv {
		finalBaseUrl = baseGovUrl
		finalBaseScimUrl = baseGovScimUrl
		finalScimVersion = ScimVersionV1
	}

	baseUrl0, err := url.Parse(finalBaseUrl)
	if err != nil {
		return nil, err
	}

	baseScimUrl0, err := url.Parse(finalBaseScimUrl)
	if err != nil {
		return nil, err
	}

	return &Client{
		baseUrl:                  baseUrl0,
		baseScimUrl:              baseScimUrl0,
		token:                    token,
		enterpriseID:             enterpriseID,
		botToken:                 botToken,
		ssoEnabled:               ssoEnabled,
		scimVersion:              finalScimVersion,
		wrapper:                  uhttp.NewBaseHttpClient(httpClient),
		workspacesNameCache:      make(map[string]string),
		workspacesNameCacheMutex: sync.RWMutex{},
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
		switch a.Error {
		case SlackErrUserAlreadyTeamMember:
			// Return an error with the exact string for the Grant function to check.
			return errors.New(SlackErrUserAlreadyTeamMember)
		case SlackErrUserAlreadyDeleted:
			// Return an error with the specific string for the Revoke function to check.
			return errors.New(SlackErrUserAlreadyDeleted)
		default:
			return fmt.Errorf(
				"baton-slack: error %s: error %v needed %v provided %v",
				action,
				a.Error,
				a.Needed,
				a.Provided,
			)
		}
	}
	return nil
}

func (c *Client) SetWorkspaceName(workspaceID, workspaceName string) {
	c.workspacesNameCacheMutex.Lock()
	defer c.workspacesNameCacheMutex.Unlock()
	c.workspacesNameCache[workspaceID] = workspaceName
}

func (c *Client) GetWorkspaceName(ctx context.Context, client *slack.Client, workspaceID string) (string, error) {
	if workspaceID == "" {
		return "", fmt.Errorf("workspace ID is empty")
	}
	c.workspacesNameCacheMutex.RLock()
	workspaceName, ok := c.workspacesNameCache[workspaceID]
	if ok {
		c.workspacesNameCacheMutex.RUnlock()
		return workspaceName, nil
	}
	c.workspacesNameCacheMutex.RUnlock()

	workspaceName = ""
	if c.enterpriseID == "" {
		nextCursor := ""
		for {
			var err error
			var workspaces []slack.Team
			params := slack.ListTeamsParameters{Cursor: nextCursor}
			workspaces, nextCursor, err = client.ListTeamsContext(ctx, params)
			if err != nil {
				return "", fmt.Errorf("error getting auth teams list: %w", err)
			}
			for _, workspace := range workspaces {
				c.SetWorkspaceName(workspace.ID, workspace.Name)
				if workspace.ID == workspaceID {
					workspaceName = workspace.Name
					nextCursor = ""
					break
				}
			}
			if nextCursor == "" {
				break
			}
		}
	} else {
		nextCursor := ""
		for {
			var err error
			var workspaces []slack.Team
			workspaces, nextCursor, _, err = c.GetAuthTeamsList(ctx, nextCursor)
			if err != nil {
				return "", fmt.Errorf("error getting auth teams list: %w", err)
			}
			for _, workspace := range workspaces {
				c.SetWorkspaceName(workspace.ID, workspace.Name)
				if workspace.ID == workspaceID {
					workspaceName = workspace.Name
					nextCursor = ""
					break
				}
			}
			if nextCursor == "" {
				break
			}
		}
	}

	if workspaceName == "" {
		return "", status.Errorf(codes.NotFound, "workspace not found: %s", workspaceID)
	}

	return workspaceName, nil
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
	urlPathIDPGroups := fmt.Sprintf(UrlPathIDPGroups, c.scimVersion)
	ratelimitData, err := c.getScim(
		ctx,
		urlPathIDPGroups,
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
		fmt.Sprintf(UrlPathIDPGroup, c.scimVersion, groupID),
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
		fmt.Sprintf(UrlPathIDPGroup, c.scimVersion, groupID),
		&response,
		payload,
	)
	if err != nil {
		return ratelimitData, fmt.Errorf("error patching IDP group: %w", err)
	}

	return ratelimitData, nil
}

func (o *Client) AddUser(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error) {
	var response BaseResponse
	ratelimitData, err := o.post(
		ctx,
		UrlPathUserAdd,
		&response,
		map[string]interface{}{
			"team_id": teamID,
			"user_id": userID,
		},
		false,
	)

	// Check for Slack API errors.
	// If the user is already a member of the team, the function returns the error "user_already_team_member".
	if err := response.handleError(err, "adding user"); err != nil {
		return ratelimitData, err
	}

	return ratelimitData, nil
}

func (o *Client) RemoveUser(ctx context.Context, teamID, userID string) (*v2.RateLimitDescription, error) {
	var response BaseResponse
	ratelimitData, err := o.post(
		ctx,
		UrlPathUserRemove,
		&response,
		map[string]interface{}{
			"team_id": teamID,
			"user_id": userID,
		},
		false,
	)

	// Check for Slack API errors.
	// If the user is already deleted, the function returns the error "user_already_deleted".
	if err := response.handleError(err, "removing user"); err != nil {
		return ratelimitData, err
	}

	return ratelimitData, nil
}

type InviteUserParams struct {
	TeamID     string
	ChannelIDs string
	Email      string
}

func (o *Client) InviteUserToWorkspace(ctx context.Context, p *InviteUserParams) (*v2.RateLimitDescription, error) {
	var response BaseResponse
	ratelimitData, err := o.post(
		ctx,
		UrlPathUserInvite,
		&response,
		map[string]interface{}{
			"team_id":     p.TeamID,
			"channel_ids": p.ChannelIDs,
			"email":       p.Email,
		},
		false, /* bot token */
	)
	return ratelimitData, response.handleError(err, "invite user")
}

// DeactivateUser deactivates a user via SCIM API using DELETE.
//
// This uses the SCIM API DELETE endpoint to deactivate a user.
// Docs: https://api.slack.com/scim
func (c *Client) DeactivateUser(
	ctx context.Context,
	userID string,
) (
	*v2.RateLimitDescription,
	error,
) {
	ratelimitData, err := c.deleteScim(
		ctx,
		fmt.Sprintf(UrlPathIDPUser, c.scimVersion, userID),
	)
	if err != nil {
		return ratelimitData, fmt.Errorf("error deactivating user: %w", err)
	}

	return ratelimitData, nil
}

// ActivateUser activates a user via SCIM API by setting active to true.
//
// This uses the SCIM API endpoint to update the user's active status.
// Docs: https://api.slack.com/scim
func (c *Client) ActivateUser(
	ctx context.Context,
	userID string,
) (
	*v2.RateLimitDescription,
	error,
) {
	type PatchOperation struct {
		Op    string      `json:"op"`
		Path  string      `json:"path"`
		Value interface{} `json:"value"`
	}

	type SCIMPatchRequest struct {
		Schemas    []string         `json:"schemas"`
		Operations []PatchOperation `json:"Operations"`
	}

	requestBody := SCIMPatchRequest{
		Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		Operations: []PatchOperation{
			{
				Op:    "replace",
				Path:  "active",
				Value: true,
			},
		},
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	var response *UserResource
	ratelimitData, err := c.patchScim(
		ctx,
		fmt.Sprintf(UrlPathIDPUser, c.scimVersion, userID),
		&response,
		payload,
	)
	if err != nil {
		return ratelimitData, fmt.Errorf("error activating user: %w", err)
	}

	return ratelimitData, nil
}

func (c *Client) AssignEnterpriseRole(
	ctx context.Context,
	roleID string,
	userID string,
	teamID string,
) (
	*v2.RateLimitDescription,
	error,
) {
	if c.enterpriseID == "" {
		return nil, fmt.Errorf("enterprise ID is required for role assignment")
	}

	var response struct {
		BaseResponse
		RejectedUsers    []string `json:"rejected_users"`
		RejectedEntities []string `json:"rejected_entities"`
	}

	entityIDs := []string{teamID}
	params := map[string]interface{}{
		"role_id":    roleID,
		"user_ids":   []string{userID},
		"entity_ids": entityIDs,
	}

	ratelimitData, err := c.postJSON(
		ctx,
		UrlPathAssignEnterpriseRole,
		&response,
		params,
		false,
	)

	if err := response.handleError(err, "assigning enterprise role"); err != nil {
		if len(response.RejectedUsers) > 0 || len(response.RejectedEntities) > 0 {
			return ratelimitData, fmt.Errorf("%w - rejected_users: %v, rejected_entities: %v", err, response.RejectedUsers, response.RejectedEntities)
		}
		return ratelimitData, err
	}
	return ratelimitData, nil
}

func (c *Client) UnassignEnterpriseRole(
	ctx context.Context,
	roleID string,
	userID string,
	teamID string,
) (
	*v2.RateLimitDescription,
	error,
) {
	if c.enterpriseID == "" {
		return nil, fmt.Errorf("enterprise ID is required for role removal")
	}

	var response struct {
		BaseResponse
		RejectedUsers    []string `json:"rejected_users"`
		RejectedEntities []string `json:"rejected_entities"`
	}

	entityIDs := []string{teamID}
	params := map[string]interface{}{
		"role_id":    roleID,
		"user_ids":   []string{userID},
		"entity_ids": entityIDs,
	}

	ratelimitData, err := c.post(
		ctx,
		UrlPathUnassignEnterpriseRole,
		&response,
		params,
		false,
	)

	if err := response.handleError(err, "unassigning enterprise role"); err != nil {
		if len(response.RejectedUsers) > 0 || len(response.RejectedEntities) > 0 {
			return ratelimitData, fmt.Errorf("%w - rejected_users: %v, rejected_entities: %v", err, response.RejectedUsers, response.RejectedEntities)
		}
		return ratelimitData, err
	}
	return ratelimitData, nil
}
