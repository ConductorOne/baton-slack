package enterprise

import (
	"fmt"
)

// docs: https://api.slack.com/methods
const (
	baseScimUrl                = "https://api.slack.com"
	baseUrl                    = "https://slack.com"
	UrlPathGetRoleAssignments  = "/api/admin.roles.listAssignments"
	UrlPathGetTeams            = "/api/admin.teams.list"
	UrlPathGetUserGroupMembers = "/api/usergroups.users.list"
	UrlPathGetUserGroups       = "/api/usergroups.list"
	UrlPathGetUserInfo         = "/api/users.info"
	UrlPathGetUsers            = "/api/users.list"
	UrlPathGetUsersAdmin       = "/api/admin.users.list"
	UrlPathIDPGroup            = "/scim/v2/Groups/%s"
	UrlPathIDPGroups           = "/scim/v2/Groups"
	UrlPathAuthTeamsList       = "/api/auth.teams.list"

	// NOTE: these are only for enterprise grid workspaces
	// docs: https://api.slack.com/methods/admin.users.setRegular
	UrlPathSetRegular = "/api/admin.users.setRegular"
	UrlPathSetAdmin   = "/api/admin.users.setAdmin"
	UrlPathSetOwner   = "/api/admin.users.setOwner"
	UrlPathUserRemove = "/api/admin.users.remove"
	UrlPathUserAdd    = "/api/admin.users.assign"
)

func getWorkspaceUrlPathByRole(roleID string) (string, error) {
	switch roleID {
	case "owner":
		return UrlPathSetOwner, nil
	case "admin":
		return UrlPathSetAdmin, nil
	case "", "member":
		return UrlPathSetRegular, nil
	default:
		return "", fmt.Errorf("invalid role type: %s", roleID)
	}
}
