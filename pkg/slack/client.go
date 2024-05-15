package enterprise

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

const baseUrl = "https://slack.com/api/"
const baseScimUrl = "https://api.slack.com/scim/v2/"

type Client struct {
	httpClient   *http.Client
	token        string
	enterpriseID string
	botToken     string
	ssoEnabled   bool
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

type SCIMResponse[T any] struct {
	Schemas      []string `json:"schemas"`
	Resources    []T      `json:"Resources"`
	TotalResults int      `json:"totalResults"`
	ItemsPerPage int      `json:"itemsPerPage"`
	StartIndex   int      `json:"startIndex"`
}

func NewClient(httpClient *http.Client, token, botToken, enterpriseID string, ssoEnabled bool) *Client {
	return &Client{
		httpClient:   httpClient,
		token:        token,
		enterpriseID: enterpriseID,
		botToken:     botToken,
		ssoEnabled:   ssoEnabled,
	}
}

// GetUserInfo returns the user info for the given user ID.
func (c *Client) GetUserInfo(ctx context.Context, userID string) (*User, error) {
	values := url.Values{
		"token": {c.botToken},
		"user":  {userID},
	}

	usersUrl, err := url.JoinPath(baseUrl, "users.info")
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
		return nil, fmt.Errorf("error fetching user info: %v", res.Error)
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

	usersUrl, err := url.JoinPath(baseUrl, "usergroups.users.list")
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
		return nil, fmt.Errorf("error fetching user group members: %v", res.Error)
	}

	return res.Users, nil
}

// GetUsers returns all users in Enterprise grid.
func (c *Client) GetUsersAdmin(ctx context.Context, cursor string) ([]UserAdmin, string, error) {
	values := url.Values{
		"token": {c.token},
	}

	// need to check if cursor is empty because API throws error if empty string is passed
	if cursor != "" {
		values.Add("cursor", cursor)
	}

	usersUrl, err := url.JoinPath(baseUrl, "admin.users.list")
	if err != nil {
		return nil, "", err
	}

	var res struct {
		BaseResponse
		Users []UserAdmin `json:"users"`
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

	usersUrl, err := url.JoinPath(baseUrl, "users.list")
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
		return nil, "", fmt.Errorf("error fetching users: %v", res.Error)
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

	teamsUrl, err := url.JoinPath(baseUrl, "admin.teams.list")
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
		return nil, "", fmt.Errorf("error fetching teams: %v", res.Error)
	}

	if res.ResponseMetadata.NextCursor != "" {
		return res.Teams, res.ResponseMetadata.NextCursor, nil
	}

	return res.Teams, "", nil
}

// GetRoleAssignments returns the role assignments for the given role ID.
func (c *Client) GetRoleAssignments(ctx context.Context, roleID string, cursor string) ([]RoleAssignment, string, error) {
	values := url.Values{
		"token": {c.token},
	}

	if roleID != "" {
		values.Add("role_ids", roleID)
	}

	if cursor != "" {
		values.Add("cursor", cursor)
	}

	teamsUrl, err := url.JoinPath(baseUrl, "admin.roles.listAssignments")
	if err != nil {
		return nil, "", err
	}

	var res struct {
		BaseResponse
		RoleAssignments []RoleAssignment `json:"role_assignments"`
		Pagination
	}

	err = c.doRequest(ctx, teamsUrl, &res, http.MethodPost, nil, values)
	if err != nil {
		return nil, "", fmt.Errorf("error fetching role assignments: %w", err)
	}

	if res.Error != "" {
		return nil, "", fmt.Errorf("error fetching role assignments: %v", res.Error)
	}

	return res.RoleAssignments, res.ResponseMetadata.NextCursor, nil
}

// GetUserGroups returns the user groups for the given team.
func (c *Client) GetUserGroups(ctx context.Context, teamID string) ([]slack.UserGroup, error) {
	// bot token needed here cause user token doesn't work unless user is in all workspaces
	values := url.Values{
		"token":   {c.botToken},
		"team_id": {teamID},
	}

	userGroupsUrl, err := url.JoinPath(baseUrl, "usergroups.list")
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
		return nil, fmt.Errorf("error fetching user groups: %v", res.Error)
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

	userGroupsUrl, err := url.JoinPath(baseUrl, "admin.users."+action)
	if err != nil {
		return err
	}

	var res BaseResponse

	err = c.doRequest(ctx, userGroupsUrl, &res, http.MethodPost, nil, values)
	if err != nil {
		return fmt.Errorf("error setting user role: %w", err)
	}

	if res.Error != "" {
		return fmt.Errorf("error setting user role: %v", res.Error)
	}

	return nil
}

// ListIDPGroups returns all IDP groups from the SCIM API.
func (c *Client) ListIDPGroups(ctx context.Context) ([]GroupResource, error) {
	groupsUrl, err := url.JoinPath(baseScimUrl, "Groups")
	if err != nil {
		return nil, err
	}

	var allGroups []GroupResource
	startIndex := 1
	count := 100

	urlParams := url.Values{}
	var res SCIMResponse[GroupResource]

	for {
		urlParams.Add("startIndex", fmt.Sprint(startIndex))
		urlParams.Add("count", fmt.Sprint(count))
		err = c.doRequest(ctx, groupsUrl, &res, http.MethodGet, nil, urlParams)
		if err != nil {
			return nil, fmt.Errorf("error fetching IDP groups: %w", err)
		}

		allGroups = append(allGroups, res.Resources...)

		startIndex += res.ItemsPerPage

		if res.TotalResults < startIndex {
			break
		}
	}

	return allGroups, nil
}

// GetIDPGroup returns a single IDP group from the SCIM API.
func (c *Client) GetIDPGroup(ctx context.Context, groupID string) (*GroupResource, error) {
	groupUrl, err := url.JoinPath(baseScimUrl, "Groups", groupID)
	if err != nil {
		return nil, err
	}

	var res GroupResource

	err = c.doRequest(ctx, groupUrl, &res, http.MethodGet, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("error fetching IDP group: %w", err)
	}

	return &res, nil
}

type PatchOp struct {
	Schemas    []string      `json:"schemas"`
	Operations []ScimOperate `json:"Operations"`
}

type ScimOperate struct {
	Op    string   `json:"op"`
	Path  string   `json:"path"`
	Value []UserID `json:"value"`
}

type UserID struct {
	Value string `json:"value"`
}

// AddUserToGroup patches a group by adding a user to it.
func (c *Client) AddUserToGroup(ctx context.Context, groupID string, user string) error {
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
func (c *Client) RemoveUserFromGroup(ctx context.Context, groupID string, user string) error {
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

func (c *Client) patchGroup(ctx context.Context, groupID string, requestBody PatchOp) error {
	groupSCIMUrl, err := url.JoinPath(baseScimUrl, "Groups", groupID)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	var res *GroupResource
	err = c.doRequest(ctx, groupSCIMUrl, &res, http.MethodPatch, payload, nil)
	if err != nil {
		return fmt.Errorf("error patching IDP group: %w", err)
	}

	return nil
}

type RateLimitError struct {
	RetryAfter time.Duration
}

func (r *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after: %s", r.RetryAfter.String())
}

func (c *Client) doRequest(ctx context.Context, url string, res interface{}, method string, payload []byte, values url.Values) error {
	l := ctxzap.Extract(ctx)
	var reqBody io.Reader

	if strings.HasPrefix(url, baseScimUrl) {
		reqBody = bytes.NewReader(payload)
	} else {
		reqBody = strings.NewReader(values.Encode())
	}

	l.Debug("making request", zap.String("method", method), zap.String("url", url))
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// different headers for SCIM API
	if strings.HasPrefix(url, baseScimUrl) {
		req.Header.Add("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")

		if values != nil {
			req.URL.RawQuery = values.Encode()
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// even though it's stated in the docs that PATCH request should return a resource
	// for some reason when adding user in IDP group response is 204, but when removing a user it returns Group resource
	if resp.StatusCode == http.StatusNoContent && method == http.MethodPatch {
		return nil
	} else {
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return err
		}
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		retryAfterSec, err := strconv.Atoi(retryAfter)
		if err != nil {
			return fmt.Errorf("error parsing retry after header: %w", err)
		}
		return &RateLimitError{RetryAfter: time.Second * time.Duration(retryAfterSec)}
	}

	return nil
}
