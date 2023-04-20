package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/slack-go/slack"
)

type roleType struct {
	DisplayName string
	Id          string
}

var roles = map[string]string{
	"primary_owner":        "Primary Owner",
	"owner":                "Owner",
	"admin":                "Admin",
	"multi_channel_guest":  "Multi Channel Guest",
	"single_channel_guest": "Single Channel Guest",
	"invited_member":       "Invited member",
	"bot":                  "Bot",
}

type workspaceRoleType struct {
	resourceType *v2.ResourceType
	client       *slack.Client
}

func (o *workspaceRoleType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func workspaceRoleBuilder(client *slack.Client) *workspaceRoleType {
	return &workspaceRoleType{
		resourceType: resourceTypeWorkspaceRole,
		client:       client,
	}
}

func (o *workspaceRoleType) List(ctx context.Context, parentResourceID *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var ret []*v2.Resource

	for roleID, roleName := range roles {
		r, err := resources.NewRoleResource(
			roleName,
			o.resourceType,
			roleID,
			nil,
			resources.WithParentResourceID(parentResourceID))
		if err != nil {
			return nil, "", nil, err
		}

		ret = append(ret, r)
	}

	return ret, "", nil, nil
}

func (o *workspaceRoleType) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement
	permissionOptions := []ent.EntitlementOption{
		ent.WithGrantableTo(resourceTypeUser),
		ent.WithDescription(fmt.Sprintf("Has the %s role in the Slack workspace", resource.DisplayName)),
		ent.WithDisplayName(fmt.Sprintf("%s Workspace Role", resource.DisplayName)),
	}

	permissionEn := ent.NewAssignmentEntitlement(resource, "assigned", permissionOptions...)
	rv = append(rv, permissionEn)
	return rv, "", nil, nil
}

func (o *workspaceRoleType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	users, err := o.client.GetUsersContext(ctx)
	if err != nil {
		return nil, "", nil, err
	}

	_, ok := roles[resource.Id.Resource]
	if !ok {
		return nil, "", nil, fmt.Errorf("unknown role %s", resource.Id.Resource)
	}

	var rv []*v2.Grant
	for _, user := range users {
		switch resource.Id.Resource {
		case "primary_owner":
			if !user.IsPrimaryOwner {
				continue
			}

		case "owner":
			if !user.IsOwner {
				continue
			}

		case "admin":
			if !user.IsAdmin {
				continue
			}

		case "multi_channel_guest":
			if !user.IsRestricted {
				continue
			}

		case "single_channel_guest":
			if !user.IsRestricted || !user.IsUltraRestricted {
				continue
			}

		case "invited_member":
			if !user.IsInvitedUser {
				continue
			}

		case "bot":
			if !user.IsBot {
				continue
			}
		}

		userID, err := resources.NewResourceID(resourceTypeUser, user.ID)
		if err != nil {
			return nil, "", nil, err
		}

		rv = append(rv, grant.NewGrant(resource, "assigned", userID))
	}

	return rv, "", nil, nil
}
