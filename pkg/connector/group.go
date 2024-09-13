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
	enterprise "github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type groupResourceType struct {
	resourceType     *v2.ResourceType
	enterpriseID     string
	enterpriseClient *enterprise.Client
	ssoEnabled       bool
}

func (g *groupResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return g.resourceType
}

func groupBuilder(enterpriseClient *enterprise.Client, enterpriseID string, ssoEnabled bool) *groupResourceType {
	return &groupResourceType{
		resourceType:     resourceTypeGroup,
		enterpriseID:     enterpriseID,
		enterpriseClient: enterpriseClient,
		ssoEnabled:       ssoEnabled,
	}
}

// Create a new connector resource for a Slack IDP group.
func groupResource(ctx context.Context, group enterprise.GroupResource) (*v2.Resource, error) {
	profile := make(map[string]interface{})
	profile["group_id"] = group.ID
	profile["group_name"] = group.DisplayName

	groupTrait := []resources.GroupTraitOption{resources.WithGroupProfile(profile)}
	ret, err := resources.NewGroupResource(group.DisplayName, resourceTypeGroup, group.ID, groupTrait)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (g *groupResourceType) List(ctx context.Context, _ *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if !g.ssoEnabled {
		return nil, "", nil, nil
	}

	groups, err := g.enterpriseClient.ListIDPGroups(ctx)
	if err != nil {
		annos, err := annotationsForError(err)
		return nil, "", annos, err
	}

	rv := make([]*v2.Resource, 0, len(groups))
	for _, group := range groups {
		cr, err := groupResource(ctx, group)
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, cr)
	}

	return rv, "", nil, nil
}

func (g *groupResourceType) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement

	assigmentOptions := []ent.EntitlementOption{
		ent.WithGrantableTo(resourceTypeUser),
		ent.WithDescription(fmt.Sprintf("Member of %s IDP group", resource.DisplayName)),
		ent.WithDisplayName(fmt.Sprintf("%s IDP group %s", resource.DisplayName, memberEntitlement)),
	}

	en := ent.NewAssignmentEntitlement(resource, memberEntitlement, assigmentOptions...)
	rv = append(rv, en)

	return rv, "", nil, nil
}

func (g *groupResourceType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var rv []*v2.Grant
	group, err := g.enterpriseClient.GetIDPGroup(ctx, resource.Id.Resource)
	if err != nil {
		annos, err := annotationsForError(err)
		return nil, "", annos, err
	}

	for _, member := range group.Members {
		userID, err := resources.NewResourceID(resourceTypeUser, member.Value)
		if err != nil {
			return nil, "", nil, err
		}

		grant := grant.NewGrant(resource, memberEntitlement, userID)
		rv = append(rv, grant)
	}

	return rv, "", nil, nil
}

func (g *groupResourceType) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	if principal.Id.ResourceType != resourceTypeUser.Id {
		l.Warn(
			"baton-slack: only users can be added to an IDP group",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-slack: only users can be added to an IDP group")
	}

	err := g.enterpriseClient.AddUserToGroup(ctx, entitlement.Resource.Id.Resource, principal.Id.Resource)
	if err != nil {
		return nil, fmt.Errorf("baton-slack: failed to add user to an IDP group: %w", err)
	}

	return nil, nil
}

func (g *groupResourceType) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	principal := grant.Principal
	entitlement := grant.Entitlement

	if principal.Id.ResourceType != resourceTypeUser.Id {
		l.Warn(
			"baton-slack: only users can be removed from an IDP group",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-slack: only users can be removed from an IDP group")
	}

	err := g.enterpriseClient.RemoveUserFromGroup(ctx, entitlement.Resource.Id.Resource, principal.Id.Resource)

	if err != nil {
		return nil, fmt.Errorf("baton-slack: failed to remove user from IDP group: %w", err)
	}

	return nil, nil
}
