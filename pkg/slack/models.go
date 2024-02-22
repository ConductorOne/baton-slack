package enterprise

import "github.com/slack-go/slack"

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
	DateCreated       int64    `json:"date_created"`
	DeactivatedTs     int64    `json:"deactivated_ts"`
	ExpirationTs      int64    `json:"expiration_ts"`
	Workspaces        []string `json:"workspaces"`
	Has2Fa            bool     `json:"has_2fa"`
	HasSso            bool     `json:"has_sso"`
}

type RoleAssignment struct {
	RoleID     string `json:"role_id"`
	EntityID   string `json:"entity_id"`
	UserID     string `json:"user_id"`
	DateCreate int64  `json:"date_create"`
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
