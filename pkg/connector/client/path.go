package enterprise

import (
	"fmt"

	"github.com/conductorone/baton-slack/pkg"
)

const (
	UrlPathGetRoleAssignments    = "/api/admin.roles.listAssignments"
	UrlPathAddRoleAssignments    = "/api/admin.roles.addAssignments"
	UrlPathRemoveRoleAssignments = "/api/admin.roles.removeAssignments"
	UrlPathGetTeams              = "/api/admin.teams.list"
	UrlPathGetUserGroupMembers   = "/api/usergroups.users.list"
	UrlPathGetUserGroups         = "/api/usergroups.list"
	UrlPathGetUserInfo           = "/api/users.info"
	UrlPathGetUsers              = "/api/users.list"
	UrlPathGetUsersAdmin         = "/api/admin.users.list"
	UrlPathIDPGroup              = "/scim/v2/Groups/%s"
	UrlPathIDPGroups             = "/scim/v2/Groups"
	UrlPathSetAdmin              = "/api/admin.users.setAdmin"
	UrlPathSetOwner              = "/api/admin.users.setOwner"
	UrlPathSetRegular            = "/api/admin.users.setRegular"
	baseScimUrl                  = "https://api.slack.com"
	baseUrl                      = "https://slack.com"
)

func getWorkspaceUrlPathByRole(roleID string) (string, error) {
	role, _ := pkg.ParseID(roleID)
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
