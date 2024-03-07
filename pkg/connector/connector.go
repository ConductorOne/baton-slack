package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	enterprise "github.com/conductorone/baton-slack/pkg/slack"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/slack-go/slack"
)

type Slack struct {
	client           *slack.Client
	apiKey           string
	enterpriseClient *enterprise.Client
	enterpriseID     string
}

var (
	resourceTypeUser = &v2.ResourceType{
		Id:          "user",
		DisplayName: "User",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_USER,
		},
	}
	resourceTypeWorkspace = &v2.ResourceType{
		Id:          "workspace",
		DisplayName: "Workspace",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_GROUP,
		},
	}
	resourceTypeUserGroup = &v2.ResourceType{
		Id:          "userGroup",
		DisplayName: "User Group",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_GROUP,
		},
	}

	resourceTypeWorkspaceRole = &v2.ResourceType{
		Id:          "workspaceRole",
		DisplayName: "Workspace Role",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_ROLE,
		},
	}
	resourceTypeEnterpriseRole = &v2.ResourceType{
		Id:          "enterpriseRole",
		DisplayName: "Enterprise Role",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_ROLE,
		},
	}
)

// Metadata returns metadata about the connector.
func (c *Slack) Metadata(ctx context.Context) (*v2.ConnectorMetadata, error) {
	return &v2.ConnectorMetadata{
		DisplayName: "Slack",
		Description: "Connector syncing users, workspaces, user groups and workspace roles from Slack to Baton.",
	}, nil
}

// Validate hits the Slack API to validate that the authenticated user has needed permissions.
func (s *Slack) Validate(ctx context.Context) (annotations.Annotations, error) {
	res, err := s.client.AuthTestContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("slack-connector: failed to authenticate. Error: %w", err)
	}

	user, err := s.client.GetUserInfoContext(ctx, res.UserID)
	if err != nil {
		return nil, fmt.Errorf("slack-connector: failed to retrieve authenticated user. Error: %w", err)
	}

	isValidUser := user.IsAdmin || user.IsOwner || user.IsPrimaryOwner || user.IsBot
	if !isValidUser {
		return nil, fmt.Errorf("slack-connector: authenticated user is not an admin, owner, primary owner or a bot")
	}
	return nil, nil
}

// New returns the Slack connector.
func New(ctx context.Context, apiKey, enterpriseKey string) (*Slack, error) {
	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, ctxzap.Extract(ctx)))
	if err != nil {
		return nil, err
	}

	client := slack.New(apiKey, slack.OptionDebug(true), slack.OptionHTTPClient(httpClient))

	res, err := client.AuthTestContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("slack-connector: failed to authenticate. Error: %w", err)
	}

	var enterpriseId string
	if res.EnterpriseID != "" {
		enterpriseId = res.EnterpriseID
	}
	enterpriseClient := enterprise.NewClient(httpClient, enterpriseKey, apiKey, res.EnterpriseID)

	return &Slack{
		client:           client,
		apiKey:           apiKey,
		enterpriseClient: enterpriseClient,
		enterpriseID:     enterpriseId,
	}, nil
}

func (s *Slack) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncer {
	return []connectorbuilder.ResourceSyncer{
		userBuilder(s.client, s.enterpriseID, s.enterpriseClient),
		workspaceBuilder(s.client, s.enterpriseID, s.enterpriseClient),
		userGroupBuilder(s.client, s.enterpriseID, s.enterpriseClient),
		workspaceRoleBuilder(s.client, s.enterpriseClient),
		enterpriseRoleBuilder(s.enterpriseID, s.enterpriseClient),
	}
}
