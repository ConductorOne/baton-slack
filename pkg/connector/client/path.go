package client

// docs: https://api.slack.com/methods
const (
	baseScimUrl                = "https://api.slack.com"
	baseGovScimUrl             = "https://api.slack-gov.com"
	baseUrl                    = "https://slack.com"
	baseGovUrl                 = "https://slack-gov.com"
	UrlPathGetUserGroupMembers = "/api/usergroups.users.list"
	UrlPathGetUserGroups       = "/api/usergroups.list"
	UrlPathGetUserInfo         = "/api/users.info"
	UrlPathGetUsers            = "/api/users.list"
	UrlPathAuthTeamsList       = "/api/auth.teams.list"
)

// all scim endpoints are only accessible with an admin scope token
//
//	https://api.slack.com/scim
//	https://docs.slack.dev/admins/scim-api/#permissions
const (
	UrlPathIDPGroup  = "/scim/%s/Groups/%s"
	UrlPathIDPGroups = "/scim/%s/Groups"
	UrlPathIDPUser   = "/scim/%s/Users/%s"
	UrlPathIDPUsers  = "/scim/%s/Users"
)
