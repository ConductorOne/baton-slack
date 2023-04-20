package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	grant "github.com/conductorone/baton-sdk/pkg/types/grant"
	resource "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/slack-go/slack"
)

const (
	primaryOwner       = "Primary Owner"
	owner              = "Owner"
	admin              = "Admin"
	member             = "Member"
	multiChannelGuest  = "Multi Channel Guest"
	signleChannelGuest = "Single Channel Guest"
	invitedMember      = "Invited member"
	bot                = "Bot"
)

var roles = []string{
	primaryOwner,
	owner,
	admin,
	member,
	multiChannelGuest,
	signleChannelGuest,
	invitedMember,
	bot,
}

type workspaceResourceType struct {
	resourceType *v2.ResourceType
	client       *slack.Client
}

func (o *workspaceResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func workspaceBuilder(client *slack.Client) *workspaceResourceType {
	return &workspaceResourceType{
		resourceType: resourceTypeWorkspace,
		client:       client,
	}
}

// Create a new connector resource for a Slack workspace.
func workspaceResource(ctx context.Context, workspace slack.Team) (*v2.Resource, error) {
	profile := make(map[string]interface{})
	profile["workspace_id"] = workspace.ID
	profile["workspace_name"] = workspace.Name
	profile["workspace_domain"] = workspace.Domain

	groupTrait := []resource.GroupTraitOption{
		resource.WithGroupProfile(profile),
	}
	workspaceOptions := []resource.ResourceOption{
		resource.WithAnnotation(
			&v2.ChildResourceType{ResourceTypeId: resourceTypeUser.Id},
			&v2.ChildResourceType{ResourceTypeId: resourceTypeUserGroup.Id},
		),
	}

	ret, err := resource.NewGroupResource(workspace.Name, resourceTypeWorkspace, workspace.ID, groupTrait, workspaceOptions...)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (o *workspaceResourceType) List(ctx context.Context, resourceId *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	bag, err := parsePageToken(pt.Token, &v2.ResourceId{ResourceType: resourceTypeWorkspace.Id})
	if err != nil {
		return nil, "", nil, err
	}

	workspaces, nextCursor, err := o.client.ListTeams(slack.ListTeamsParameters{Cursor: bag.PageToken()})
	if err != nil {
		return nil, "", nil, err
	}

	pageToken, err := bag.NextToken(nextCursor)
	if err != nil {
		return nil, "", nil, err
	}

	rv := make([]*v2.Resource, 0, len(workspaces))
	for _, workspace := range workspaces {
		wr, err := workspaceResource(ctx, workspace)
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, wr)
	}

	return rv, pageToken, nil, nil
}

func (o *workspaceResourceType) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement
	for _, role := range roles {
		permissionOptions := []ent.EntitlementOption{
			ent.WithGrantableTo(resourceTypeUser),
			ent.WithDescription(fmt.Sprintf("Role in %s Slack workspace", resource.DisplayName)),
			ent.WithDisplayName(fmt.Sprintf("%s Workspace %s", resource.DisplayName, role)),
		}

		permissionEn := ent.NewPermissionEntitlement(resource, role, permissionOptions...)
		rv = append(rv, permissionEn)
	}
	return rv, "", nil, nil
}

func (o *workspaceResourceType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	users, err := o.client.GetUsersContext(ctx)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Grant

	for _, user := range users {
		var roleName string
		switch {
		case user.IsPrimaryOwner:
			roleName = primaryOwner
		case user.IsOwner:
			roleName = owner
		case user.IsAdmin:
			roleName = admin
		case user.IsRestricted:
			roleName = multiChannelGuest
		case user.IsUltraRestricted:
			roleName = signleChannelGuest
		case user.IsInvitedUser:
			roleName = invitedMember
		case user.IsBot:
			roleName = bot
		default:
			roleName = member
		}
		userCopy := user
		ur, err := userResource(ctx, &userCopy, resource.Id)
		if err != nil {
			return nil, "", nil, err
		}

		permissionGrant := grant.NewGrant(resource, roleName, ur.Id)
		rv = append(rv, permissionGrant)
	}

	return rv, "", nil, nil
}
