package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
)

func capabilityPermissions(perms ...string) *v2.CapabilityPermissions {
	cp := &v2.CapabilityPermissions{}
	for _, p := range perms {
		cp.Permissions = append(cp.Permissions, &v2.CapabilityPermission{Permission: p})
	}
	return cp
}

var (
	resourceTypeUser = &v2.ResourceType{
		Id:          "user",
		DisplayName: "User",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_USER,
		},
		Annotations: annotations.New(
			&v2.SkipEntitlementsAndGrants{},
			capabilityPermissions(
				// Bot Token Scopes
				"users:read",
				"users:read.email",
				"users.profile:read",
				// User Token Scopes (Business+ SCIM)
				"admin",
				"admin.users:read",
			),
		),
	}
	resourceTypeWorkspace = &v2.ResourceType{
		Id:          "workspace",
		DisplayName: "Workspace",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_GROUP,
		},
		Annotations: annotations.New(
			capabilityPermissions(
				// Bot Token Scopes
				"team:read",
				"users:read",
				"users:read.email",
				"channels:join",
				"channels:read",
				"groups:read",
				// User Token Scopes (Business+)
				"admin.teams:read",
			),
		),
	}
	resourceTypeUserGroup = &v2.ResourceType{
		Id:          "userGroup",
		DisplayName: "User Group",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_GROUP,
		},
		Annotations: annotations.New(
			capabilityPermissions(
				// Bot Token Scopes
				"usergroups:read",
				"users:read",
				// User Token Scopes (Business+)
				"admin.usergroups:read",
			),
		),
	}
	resourceTypeGroup = &v2.ResourceType{
		Id:          "group",
		DisplayName: "IDP Group",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_GROUP,
		},
		Annotations: annotations.New(
			capabilityPermissions(
				// User Token Scopes (Business+ SCIM)
				"admin",
				"admin.users:write",
			),
		),
	}

	resourceTypeWorkspaceRole = &v2.ResourceType{
		Id:          "workspaceRole",
		DisplayName: "Workspace Role",
		Annotations: annotations.New(
			&v2.SkipGrants{},
			capabilityPermissions(
				// User Token Scopes (Business+)
				"admin",
				"admin.roles:read",
			),
		),
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_ROLE,
		},
	}
)
