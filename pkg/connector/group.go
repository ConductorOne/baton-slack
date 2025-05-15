package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-slack/pkg"
	enterprise "github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

// TODO(marcos): Is this actually a bug?
const StartingOffset = 1

type groupResourceType struct {
	resourceType     *v2.ResourceType
	enterpriseID     string
	enterpriseClient *enterprise.Client
	ssoEnabled       bool
	govEnv           bool
}

func (g *groupResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return g.resourceType
}

func groupBuilder(enterpriseClient *enterprise.Client, enterpriseID string, ssoEnabled bool, govEnv bool) *groupResourceType {
	return &groupResourceType{
		resourceType:     resourceTypeGroup,
		enterpriseID:     enterpriseID,
		enterpriseClient: enterpriseClient,
		ssoEnabled:       ssoEnabled,
		govEnv:           govEnv,
	}
}

// Create a new connector resource for a Slack IDP group.
func groupResource(
	_ context.Context,
	group enterprise.GroupResource,
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
func parsePaginationToken(pToken *pagination.Token) (int, int, error) {
	var (
		limit  = enterprise.PageSizeDefault
		offset = StartingOffset
	)

	if pToken != nil {
		if pToken.Size > 0 {
			limit = pToken.Size
		}

		if pToken.Token != "" {
			parsedOffset, err := strconv.Atoi(pToken.Token)
			if err != nil {
				return 0, 0, err
			}
			offset = parsedOffset
		}
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
	pageToken *pagination.Token,
) (
	[]*v2.Resource,
	string,
	annotations.Annotations,
	error,
) {
	if !g.ssoEnabled {
		return nil, "", nil, nil
	}

	offset, limit, err := parsePaginationToken(pageToken)
	if err != nil {
		return nil, "", nil, err
	}

	outputAnnotations := annotations.New()
	groupsResponse, ratelimitData, err := g.enterpriseClient.ListIDPGroups(ctx, offset, limit)
	outputAnnotations.WithRateLimiting(ratelimitData)
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	groups, err := pkg.MakeResourceList(
		ctx,
		groupsResponse.Resources,
		parentResourceId,
		groupResource,
	)
	if err != nil {
		return nil, "", nil, err
	}

	nextToken := getNextToken(offset, limit, groupsResponse.TotalResults)

	return groups, nextToken, outputAnnotations, nil
}

func (g *groupResourceType) Entitlements(
	ctx context.Context,
	resource *v2.Resource,
	_ *pagination.Token,
) (
	[]*v2.Entitlement,
	string,
	annotations.Annotations,
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
		"",
		nil,
		nil
}

func (g *groupResourceType) Grants(
	ctx context.Context,
	resource *v2.Resource,
	_ *pagination.Token,
) (
	[]*v2.Grant,
	string,
	annotations.Annotations,
	error,
) {
	outputAnnotations := annotations.New()

	var rv []*v2.Grant
	group, ratelimitData, err := g.enterpriseClient.GetIDPGroup(ctx, resource.Id.Resource)
	outputAnnotations.WithRateLimiting(ratelimitData)
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	for _, member := range group.Members {
		userID, err := resources.NewResourceID(resourceTypeUser, member.Value)
		if err != nil {
			return nil, "", nil, err
		}
		grantOptions := []grant.GrantOption{}
		if g.govEnv {
			grantOptions = append(grantOptions, grant.WithAnnotation(&v2.GrantImmutable{}))
		}

		grant := grant.NewGrant(resource, memberEntitlement, userID, grantOptions...)
		rv = append(rv, grant)
	}

	return rv, "", outputAnnotations, nil
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

	if g.govEnv {
		logger.Debug(
			"baton-slack: IDP group provisioning is not supported in Gov environment",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-slack: IDP group provisioning is not supported in Gov environment")
	}

	if principal.Id.ResourceType != resourceTypeUser.Id {
		logger.Warn(
			"baton-slack: only users can be added to an IDP group",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-slack: only users can be added to an IDP group")
	}

	outputAnnotations := annotations.New()
	ratelimitData, err := g.enterpriseClient.AddUserToGroup(
		ctx,
		entitlement.Resource.Id.Resource,
		principal.Id.Resource,
	)
	outputAnnotations.WithRateLimiting(ratelimitData)
	if err != nil {
		return outputAnnotations, fmt.Errorf("baton-slack: failed to add user to an IDP group: %w", err)
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

	if g.govEnv {
		logger.Debug(
			"baton-slack: IDP group provisioning is not supported in Gov environment",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-slack: IDP group provisioning is not supported in Gov environment")
	}

	if principal.Id.ResourceType != resourceTypeUser.Id {
		logger.Warn(
			"baton-slack: only users can be removed from an IDP group",
			zap.String("principal_type", principal.Id.ResourceType),
			zap.String("principal_id", principal.Id.Resource),
		)
		return nil, fmt.Errorf("baton-slack: only users can be removed from an IDP group")
	}

	outputAnnotations := annotations.New()
	wasRevoked, ratelimitData, err := g.enterpriseClient.RemoveUserFromGroup(
		ctx,
		entitlement.Resource.Id.Resource,
		principal.Id.Resource,
	)
	outputAnnotations.WithRateLimiting(ratelimitData)

	if err != nil {
		return outputAnnotations, fmt.Errorf("baton-slack: failed to remove user from IDP group: %w", err)
	}

	if !wasRevoked {
		outputAnnotations.Append(&v2.GrantAlreadyRevoked{})
	}

	return outputAnnotations, nil
}
