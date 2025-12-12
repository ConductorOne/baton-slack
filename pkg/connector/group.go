package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"

	"github.com/conductorone/baton-slack/pkg"
	"github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

// TODO(marcos): Is this actually a bug?
const StartingOffset = 1

type groupResourceType struct {
	resourceType       *v2.ResourceType
	businessPlusClient *client.Client
	govEnv             bool
}

func (g *groupResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return g.resourceType
}

func groupBuilder(businessPlusClient *client.Client, govEnv bool) *groupResourceType {
	return &groupResourceType{
		resourceType:       resourceTypeGroup,
		businessPlusClient: businessPlusClient,
		govEnv:             govEnv,
	}
}

// Create a new connector resource for a Slack IDP group.
func groupResource(
	_ context.Context,
	group client.GroupResource,
	_ *v2.ResourceId,
) (*v2.Resource, error) {
	return resources.NewGroupResource(
		group.DisplayName,
		resourceTypeGroup,
		group.ID,
		[]resources.GroupTraitOption{
			resources.WithGroupProfile(
				map[string]interface{}{
					"group_id":   group.ID,
					"group_name": group.DisplayName,
				},
			),
		},
	)
}

// parsePaginationToken - takes as pagination token and returns offset and limit
// in that order. TODO(marcos): move this to a util.
func parsePaginationToken(tokenStr string, tokenSize int) (int, int, error) {
	var (
		limit  = client.PageSizeDefault
		offset = StartingOffset
	)

	if tokenSize > 0 {
		limit = tokenSize
	}

	if tokenStr != "" {
		parsedOffset, err := strconv.Atoi(tokenStr)
		if err != nil {
			return 0, 0, err
		}
		offset = parsedOffset
	}

	return offset, limit, nil
}

func getNextToken(offset int, limit int, total int) string {
	nextOffset := offset + limit
	if nextOffset > total {
		return ""
	}
	return strconv.Itoa(nextOffset)
}

func (g *groupResourceType) List(
	ctx context.Context,
	parentResourceId *v2.ResourceId,
	attrs resources.SyncOpAttrs,
) (
	[]*v2.Resource,
	*resources.SyncOpResults,
	error,
) {
	l := ctxzap.Extract(ctx)
	if g.businessPlusClient == nil {
		l.Debug("Business+ client not available, skipping IDP groups")
		return nil, &resources.SyncOpResults{}, nil
	}

	offset, limit, err := parsePaginationToken(attrs.PageToken.Token, attrs.PageToken.Size)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing pagination token: %w", err)
	}

	outputAnnotations := annotations.New()
	groupsResponse, ratelimitData, err := g.businessPlusClient.ListIDPGroups(ctx, offset, limit)
	outputAnnotations.WithRateLimiting(ratelimitData)
	if err != nil {
		return nil, &resources.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("listing IDP groups: %w", err)
	}

	groups, err := pkg.MakeResourceList(
		ctx,
		groupsResponse.Resources,
		parentResourceId,
		groupResource,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating group resources: %w", err)
	}

	nextToken := getNextToken(offset, limit, groupsResponse.TotalResults)

	return groups, &resources.SyncOpResults{NextPageToken: nextToken, Annotations: outputAnnotations}, nil
}

func (g *groupResourceType) Entitlements(
	ctx context.Context,
	resource *v2.Resource,
	_ resources.SyncOpAttrs,
) (
	[]*v2.Entitlement,
	*resources.SyncOpResults,
	error,
) {
	return []*v2.Entitlement{
			entitlement.NewAssignmentEntitlement(
				resource,
				memberEntitlement,
				entitlement.WithGrantableTo(resourceTypeUser),
				entitlement.WithDescription(
					fmt.Sprintf(
						"Member of %s IDP group",
						resource.DisplayName,
					),
				),
				entitlement.WithDisplayName(
					fmt.Sprintf(
						"%s IDP group %s",
						resource.DisplayName,
						memberEntitlement,
					),
				),
			),
		},
		&resources.SyncOpResults{},
		nil
}

func (g *groupResourceType) Grants(
	ctx context.Context,
	resource *v2.Resource,
	_ resources.SyncOpAttrs,
) (
	[]*v2.Grant,
	*resources.SyncOpResults,
	error,
) {
	outputAnnotations := annotations.New()

	var rv []*v2.Grant
	group, ratelimitData, err := g.businessPlusClient.GetIDPGroup(ctx, resource.Id.Resource)
	outputAnnotations.WithRateLimiting(ratelimitData)
	if err != nil {
		return nil, &resources.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("fetching IDP group: %w", err)
	}

	for _, member := range group.Members {
		userID, err := resources.NewResourceID(resourceTypeUser, member.Value)
		if err != nil {
			return nil, nil, fmt.Errorf("creating user resource ID: %w", err)
		}
		grantOptions := []grant.GrantOption{}
		if g.govEnv {
			grantOptions = append(grantOptions, grant.WithAnnotation(&v2.GrantImmutable{}))
		}

		grant := grant.NewGrant(resource, memberEntitlement, userID, grantOptions...)
		rv = append(rv, grant)
	}

	return rv, &resources.SyncOpResults{Annotations: outputAnnotations}, nil
}

func (g *groupResourceType) Grant(
	ctx context.Context,
	principal *v2.Resource,
	entitlement *v2.Entitlement,
) (
	annotations.Annotations,
	error,
) {
	logger := ctxzap.Extract(ctx)

	if g.businessPlusClient == nil {
		return nil, fmt.Errorf("business+ client not available: missing Business+ token")
	}

	if g.govEnv {
		logger.Debug(
			"baton-slack: IDP group provisioning is not supported in Gov environment",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("IDP group provisioning not supported in Gov environment for grant operation")
	}

	if principal.Id.ResourceType != resourceTypeUser.Id {
		logger.Warn(
			"baton-slack: only users can be added to an IDP group",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("only users can be granted IDP group membership")
	}

	outputAnnotations := annotations.New()
	ratelimitData, err := g.businessPlusClient.AddUserToGroup(
		ctx,
		entitlement.Resource.Id.Resource,
		principal.Id.Resource,
	)
	outputAnnotations.WithRateLimiting(ratelimitData)
	if err != nil {
		return outputAnnotations, fmt.Errorf("adding user to IDP group: %w", err)
	}

	return outputAnnotations, nil
}

func (g *groupResourceType) Revoke(
	ctx context.Context,
	grant *v2.Grant,
) (
	annotations.Annotations,
	error,
) {
	logger := ctxzap.Extract(ctx)

	principal := grant.Principal
	entitlement := grant.Entitlement

	if g.businessPlusClient == nil {
		return nil, fmt.Errorf("business+ client not available: missing Business+ token")
	}

	if g.govEnv {
		logger.Debug(
			"baton-slack: IDP group provisioning is not supported in Gov environment",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("provisioning of IDP group not supported in Gov environment for revoke operation")
	}

	if principal.Id.ResourceType != resourceTypeUser.Id {
		logger.Warn(
			"baton-slack: only users can be removed from an IDP group",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("only users can have IDP group membership revoked")
	}

	outputAnnotations := annotations.New()
	wasRevoked, ratelimitData, err := g.businessPlusClient.RemoveUserFromGroup(
		ctx,
		entitlement.Resource.Id.Resource,
		principal.Id.Resource,
	)
	outputAnnotations.WithRateLimiting(ratelimitData)

	if err != nil {
		return outputAnnotations, fmt.Errorf("removing user from IDP group: %w", err)
	}

	if !wasRevoked {
		outputAnnotations.Append(&v2.GrantAlreadyRevoked{})
	}

	return outputAnnotations, nil
}
