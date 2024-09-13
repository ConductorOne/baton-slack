package enterprise

import (
	"fmt"
	"strings"
)

const (
	UrlPathGetRoleAssignments  = "admin.roles.listAssignments"
	UrlPathGetTeams            = "admin.teams.list"
	UrlPathGetUserGroupMembers = "usergroups.users.list"
	UrlPathGetUserGroups       = "usergroups.list"
	UrlPathGetUserInfo         = "users.info"
	UrlPathGetUsers            = "users.list"
	UrlPathGetUsersAdmin       = "admin.users.list"
	UrlPathIDPGroup            = "Groups/%s"
	UrlPathIDPGroups           = "Groups"
	UrlPathSetAdmin            = "admin.users.setAdmin"
	UrlPathSetOwner            = "admin.users.setOwner"
	UrlPathSetRegular          = "admin.users.setRegular"
	baseScimUrl                = "https://api.slack.com/scim/v2/"
	baseUrl                    = "https://slack.com/api/"
)

func getWorkspaceUrlPathByRole(roleID string) (string, error) {
	var role string

	if roleID != "" {
		roleSplit := strings.Split(roleID, ":")
		if len(roleSplit) >= 2 {
			role = roleSplit[1]
		}
	}

	switch role {
	case "owner":
		return UrlPathSetOwner, nil
	case "admin":
		return UrlPathSetAdmin, nil
	case "":
		return UrlPathSetRegular, nil
	default:
		return "", fmt.Errorf("invalid role type: %s", role)
	}
}
