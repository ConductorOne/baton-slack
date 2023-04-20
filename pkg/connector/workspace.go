package connector

import (
	"context"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	resource "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/slack-go/slack"
)

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
			&v2.ChildResourceType{ResourceTypeId: resourceTypeWorkspaceRole.Id},
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
	return nil, "", nil, nil
}

func (o *workspaceResourceType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}
