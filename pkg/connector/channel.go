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

var memberEntitlement = "member"

type channelResourceType struct {
	resourceType *v2.ResourceType
	client       *slack.Client
	channels     []string
}

func (o *channelResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func channelBuilder(client *slack.Client, channels []string) *channelResourceType {
	return &channelResourceType{
		resourceType: resourceTypeChannel,
		client:       client,
		channels:     channels,
	}
}

func (o *channelResourceType) List(ctx context.Context, parentResourceID *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	var defaultChannels []string

	userGroups, err := o.client.GetUserGroupsContext(ctx)
	if err != nil {
		return nil, "", nil, err
	}

	if o.channels != nil {
		defaultChannels = append(defaultChannels, o.channels...)
	}

	for _, userGroup := range userGroups {
		defaultChannels = append(defaultChannels, userGroup.Prefs.Channels...)
		defaultChannels = append(defaultChannels, userGroup.Prefs.Groups...)
	}

	rv := make([]*v2.Resource, 0, len(defaultChannels))
	for _, defaultChannel := range defaultChannels {
		channel, err := o.client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{ChannelID: defaultChannel})
		if err != nil {
			return nil, "", nil, err
		}
		cr, err := channelResource(ctx, *channel, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, cr)
	}
	return rv, "", nil, nil
}

func (o *channelResourceType) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement

	assigmentOptions := []ent.EntitlementOption{
		ent.WithGrantableTo(resourceTypeUser),
		ent.WithDescription(fmt.Sprintf("Member of %s Slack channel", resource.DisplayName)),
		ent.WithDisplayName(fmt.Sprintf("%s Channel %s", resource.DisplayName, memberEntitlement)),
	}

	en := ent.NewAssignmentEntitlement(resource, memberEntitlement, assigmentOptions...)
	rv = append(rv, en)

	return rv, "", nil, nil
}

func (o *channelResourceType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var rv []*v2.Grant

	channel, err := o.client.GetConversationInfo(&slack.GetConversationInfoInput{ChannelID: resource.Id.Resource})
	if err != nil {
		return nil, "", nil, err
	}

	members, _, err := o.client.GetUsersInConversationContext(ctx, &slack.GetUsersInConversationParameters{ChannelID: channel.ID})
	if err != nil {
		return nil, "", nil, err
	}

	for _, member := range members {
		userInfo, err := o.client.GetUserInfoContext(ctx, member)
		if err != nil {
			return nil, "", nil, err
		}

		ur, err := userResource(ctx, userInfo, resource.Id)
		if err != nil {
			return nil, "", nil, err
		}

		grant := grant.NewGrant(resource, memberEntitlement, ur.Id)
		rv = append(rv, grant)
	}

	return rv, "", nil, nil
}

func channelResource(ctx context.Context, channel slack.Channel, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := make(map[string]interface{})
	profile["channel_id"] = channel.ID
	profile["channel_name"] = channel.Name

	groupTrait := []resource.GroupTraitOption{resource.WithGroupProfile(profile)}
	ret, err := resource.NewGroupResource(channel.Name, resourceTypeChannel, channel.ID, groupTrait, resource.WithParentResourceID(parentResourceID))
	if err != nil {
		return nil, err
	}

	return ret, nil
}
