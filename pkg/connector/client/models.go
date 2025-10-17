package enterprise

import "github.com/slack-go/slack"

type BaseResponse struct {
	Ok       bool   `json:"ok"`
	Error    string `json:"error"`
	Needed   string `json:"needed"`
	Provided string `json:"provided"`
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

type UserAdmin struct {
	ID                string   `json:"id"`
	Email             string   `json:"email"`
	IsAdmin           bool     `json:"is_admin"`
	IsOwner           bool     `json:"is_owner"`
	IsPrimaryOwner    bool     `json:"is_primary_owner"`
	IsRestricted      bool     `json:"is_restricted"`
	IsUltraRestricted bool     `json:"is_ultra_restricted"`
	IsBot             bool     `json:"is_bot"`
	Username          string   `json:"username"`
	FullName          string   `json:"full_name"`
	IsActive          bool     `json:"is_active"`
	DateCreated       int      `json:"date_created"`
	DeactivatedTs     int      `json:"deactivated_ts"`
	ExpirationTs      int      `json:"expiration_ts"`
	Workspaces        []string `json:"workspaces"`
	Has2Fa            bool     `json:"has_2fa"`
	HasSso            bool     `json:"has_sso"`
}

type RoleAssignment struct {
	RoleID     string `json:"role_id"`
	EntityID   string `json:"entity_id"`
	UserID     string `json:"user_id"`
	DateCreate int    `json:"date_create"`
}

type User struct {
	ID                string            `json:"id"`
	TeamID            string            `json:"team_id"`
	Name              string            `json:"name"`
	Deleted           bool              `json:"deleted"`
	Color             string            `json:"color"`
	RealName          string            `json:"real_name"`
	TZ                string            `json:"tz,omitempty"`
	TZLabel           string            `json:"tz_label"`
	TZOffset          int               `json:"tz_offset"`
	Profile           slack.UserProfile `json:"profile"`
	IsBot             bool              `json:"is_bot"`
	IsAdmin           bool              `json:"is_admin"`
	IsOwner           bool              `json:"is_owner"`
	IsPrimaryOwner    bool              `json:"is_primary_owner"`
	IsRestricted      bool              `json:"is_restricted"`
	IsUltraRestricted bool              `json:"is_ultra_restricted"`
	IsStranger        bool              `json:"is_stranger"`
	IsAppUser         bool              `json:"is_app_user"`
	IsInvitedUser     bool              `json:"is_invited_user"`
	Has2FA            bool              `json:"has_2fa"`
	TwoFactorType     string            `json:"two_factor_type"`
	HasFiles          bool              `json:"has_files"`
	Presence          string            `json:"presence"`
	Locale            string            `json:"locale"`
	Enterprise        EnterpriseUser    `json:"enterprise_user,omitempty"`
}

type EnterpriseUser struct {
	ID             string   `json:"id"`
	EnterpriseID   string   `json:"enterprise_id"`
	EnterpriseName string   `json:"enterprise_name"`
	IsAdmin        bool     `json:"is_admin"`
	IsOwner        bool     `json:"is_owner"`
	IsPrimaryOwner bool     `json:"is_primary_owner"`
	Teams          []string `json:"teams"`
}

// UserResource SCIM resources.
type UserResource struct {
	Schemas                                           []string                                               `json:"schemas"`
	ID                                                string                                                 `json:"id"`
	ExternalID                                        string                                                 `json:"externalId"`
	Meta                                              Meta                                                   `json:"meta"`
	UserName                                          string                                                 `json:"userName"`
	NickName                                          string                                                 `json:"nickName"`
	Name                                              Name                                                   `json:"name"`
	DisplayName                                       string                                                 `json:"displayName"`
	ProfileURL                                        string                                                 `json:"profileUrl"`
	Title                                             string                                                 `json:"title"`
	Timezone                                          string                                                 `json:"timezone"`
	Active                                            bool                                                   `json:"active"`
	Emails                                            []Email                                                `json:"emails"`
	Addresses                                         []Address                                              `json:"addresses"`
	PhoneNumbers                                      []PhoneNumber                                          `json:"phoneNumbers"`
	UserType                                          string                                                 `json:"userType"`
	Roles                                             []Email                                                `json:"roles"`
	PreferredLanguage                                 string                                                 `json:"preferredLanguage"`
	Locale                                            string                                                 `json:"locale"`
	UrnIETFParamsScimSchemasExtensionEnterprise20User UrnIETFParamsScimSchemasExtensionEnterprise20UserClass `json:"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"`
	Groups                                            []Group                                                `json:"groups"`
}
type Address struct {
	StreetAddress string `json:"streetAddress"`
	Locality      string `json:"locality"`
	Region        string `json:"region"`
	PostalCode    string `json:"postalCode"`
	Country       string `json:"country"`
}

type Email struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary"`
}

type Group struct {
	Display string `json:"display"`
	Value   string `json:"value"`
}

type Meta struct {
	Created  string `json:"created"`
	Location string `json:"location"`
}

type Name struct {
	GivenName       string `json:"givenName"`
	FamilyName      string `json:"familyName"`
	HonorificPrefix string `json:"honorificPrefix"`
}

type PhoneNumber struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

type UrnIETFParamsScimSchemasExtensionEnterprise20UserClass struct {
	EmployeeNumber string  `json:"employeeNumber"`
	CostCenter     string  `json:"costCenter"`
	Organization   string  `json:"organization"`
	Division       string  `json:"division"`
	Department     string  `json:"department"`
	Manager        Manager `json:"manager"`
}

type Manager struct {
	ManagerID string `json:"managerId"`
}

type Member struct {
	Value   string `json:"value"`
	Display string `json:"display"`
}

type GroupResource struct {
	Schemas     []string `json:"schemas"`
	ID          string   `json:"id"`
	Meta        Meta     `json:"meta"`
	DisplayName string   `json:"displayName"`
	Members     []Member `json:"members"`
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
