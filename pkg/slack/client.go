package enterprise

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/slack-go/slack"
)

const baseUrl = "https://slack.com/api/"

type Client struct {
	httpClient   *http.Client
	token        string
	enterpriseID string
	botToken     string
}

type BaseResponse struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error"`
}

type Pagination struct {
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

func NewClient(httpClient *http.Client, token, botToken, enterpriseID string) *Client {
	return &Client{
		httpClient:   httpClient,
		token:        token,
		enterpriseID: enterpriseID,
		botToken:     botToken,
	}
}

// GetUserInfo returns the user info for the given user ID.
func (c *Client) GetUserInfo(ctx context.Context, userID string) (*User, error) {
	values := url.Values{
		"token": {c.botToken},
		"user":  {userID},
	}

	usersUrl, err := url.JoinPath(baseUrl + "users.info")
	if err != nil {
		return nil, err
	}

	var res struct {
		BaseResponse
		User *User `json:"user"`
	}

	err = c.doRequest(ctx, usersUrl, &res, http.MethodPost, nil, values)
	if err != nil {
		return nil, fmt.Errorf("error fetching user info: %w", err)
	}

	if res.Error != "" {
		return nil, fmt.Errorf(res.Error)
	}

	return res.User, nil
}

// GetUserGroupMembers returns the members of the given user group from a given team.
func (c *Client) GetUserGroupMembers(ctx context.Context, userGroupID, teamID string) ([]string, error) {
	values := url.Values{
		"token":     {c.botToken},
		"team_id":   {teamID},
		"usergroup": {userGroupID},
	}

	usersUrl, err := url.JoinPath(baseUrl + "usergroups.users.list")
	if err != nil {
		return nil, err
	}

	var res struct {
		BaseResponse
		Users []string `json:"users"`
	}

	err = c.doRequest(ctx, usersUrl, &res, http.MethodPost, nil, values)
	if err != nil {
		return nil, fmt.Errorf("error fetching user group members: %w", err)
	}

	if res.Error != "" {
		return nil, fmt.Errorf(res.Error)
	}

	return res.Users, nil
}

// GetUsers returns the users of the given team.
func (c *Client) GetUsers(ctx context.Context, teamID, cursor string) ([]User, string, error) {
	values := url.Values{
		"token":   {c.botToken},
		"team_id": {teamID},
	}

	// need to check if cursor is empty because API throws error if empty string is passed
	if cursor != "" {
		values.Add("cursor", cursor)
	}

	usersUrl, err := url.JoinPath(baseUrl + "users.list")
	if err != nil {
		return nil, "", err
	}

	var res struct {
		BaseResponse
		Users []User `json:"members"`
		Pagination
	}

	err = c.doRequest(ctx, usersUrl, &res, http.MethodPost, nil, values)
	if err != nil {
		return nil, "", fmt.Errorf("error fetching users: %w", err)
	}

	if res.Error != "" {
		return nil, "", fmt.Errorf(res.Error)
	}

	if res.ResponseMetadata.NextCursor != "" {
		return res.Users, res.ResponseMetadata.NextCursor, nil
	}

	return res.Users, "", nil
}

// GetTeams returns the teams of the given enterprise.
func (c *Client) GetTeams(ctx context.Context, cursor string) ([]slack.Team, string, error) {
	values := url.Values{
		"token": {c.token},
	}

	if cursor != "" {
		values.Add("cursor", cursor)
	}

	teamsUrl, err := url.JoinPath(baseUrl + "admin.teams.list")
	if err != nil {
		return nil, "", err
	}

	var res struct {
		BaseResponse
		Teams []slack.Team `json:"teams"`
		Pagination
	}

	err = c.doRequest(ctx, teamsUrl, &res, http.MethodPost, nil, values)
	if err != nil {
		return nil, "", fmt.Errorf("error fetching teams: %w", err)
	}

	if res.Error != "" {
		return nil, "", fmt.Errorf(res.Error)
	}

	if res.ResponseMetadata.NextCursor != "" {
		return res.Teams, res.ResponseMetadata.NextCursor, nil
	}

	return res.Teams, "", nil
}

// GetRoleAssignments returns the role assignments for the given role ID.
func (c *Client) GetRoleAssignments(ctx context.Context, roleID string) ([]RoleAssignment, error) {
	values := url.Values{
		"token":    {c.token},
		"role_ids": {roleID},
	}

	teamsUrl, err := url.JoinPath(baseUrl + "admin.roles.listAssignments")
	if err != nil {
		return nil, err
	}

	var res struct {
		BaseResponse
		RoleAssignments []RoleAssignment `json:"role_assignments"`
	}

	err = c.doRequest(ctx, teamsUrl, &res, http.MethodPost, nil, values)
	if err != nil {
		return nil, fmt.Errorf("error fetching role assignments: %w", err)
	}

	if res.Error != "" {
		return nil, fmt.Errorf(res.Error)
	}

	return res.RoleAssignments, nil
}

// GetUserGroups returns the user groups for the given team.
func (c *Client) GetUserGroups(ctx context.Context, teamID string) ([]slack.UserGroup, error) {
	// bot token needed here cause user token doesn't work unless user is in all workspaces
	values := url.Values{
		"token":   {c.botToken},
		"team_id": {teamID},
	}

	userGroupsUrl, err := url.JoinPath(baseUrl + "usergroups.list")
	if err != nil {
		return nil, err
	}

	var res struct {
		BaseResponse
		UserGroups []slack.UserGroup `json:"usergroups"`
	}

	err = c.doRequest(ctx, userGroupsUrl, &res, http.MethodPost, nil, values)
	if err != nil {
		return nil, fmt.Errorf("error fetching user groups: %w", err)
	}

	if res.Error != "" {
		return nil, fmt.Errorf(res.Error)
	}

	return res.UserGroups, nil
}

// SetWorkspaceRole sets the role for the given user in the given team.
func (c *Client) SetWorkspaceRole(ctx context.Context, teamID, userID, roleID string) error {
	values := url.Values{
		"token":   {c.token},
		"team_id": {teamID},
		"user_id": {userID},
	}

	var role string

	if roleID != "" {
		roleSplit := strings.Split(roleID, ":")
		if len(roleSplit) >= 2 {
			role = roleSplit[1]
		}
	}

	var action string
	switch role {
	case "owner":
		action = "setOwner"
	case "admin":
		action = "setAdmin"
	case "":
		action = "setRegular"
	default:
		return fmt.Errorf("invalid role type: %s", role)
	}

	userGroupsUrl, err := url.JoinPath(baseUrl + "admin.users." + action)
	if err != nil {
		return err
	}

	var res BaseResponse

	err = c.doRequest(ctx, userGroupsUrl, &res, http.MethodPost, nil, values)
	if err != nil {
		return fmt.Errorf("error setting user role: %w", err)
	}

	if res.Error != "" {
		return fmt.Errorf(res.Error)
	}

	return nil
}

func (c *Client) doRequest(ctx context.Context, url string, res interface{}, method string, payload []byte, values url.Values) error {
	reqBody := strings.NewReader(values.Encode())

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	return nil
}
