package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	enterprise "github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

type Slack struct {
	client           *slack.Client
	apiKey           string
	enterpriseClient *enterprise.Client
	enterpriseID     string
	ssoEnabled       bool
}

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

type slackLogger struct {
	ZapLog *zap.Logger
}

// Output Needed to prevent slack client from logging in its own format.
func (s *slackLogger) Output(callDepth int, msg string) error {
	s.ZapLog.Info(msg, zap.Int("callDepth", callDepth))
	return nil
}

// New returns the Slack connector.
func New(ctx context.Context, apiKey, enterpriseKey string, ssoEnabled bool) (*Slack, error) {
	l := ctxzap.Extract(ctx)
	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, l))
	if err != nil {
		return nil, err
	}

	logger := &slackLogger{ZapLog: l}
	opts := []slack.Option{
		slack.OptionDebug(true),
		slack.OptionHTTPClient(httpClient),
		slack.OptionLog(logger),
	}
	client := slack.New(apiKey, opts...)

	res, err := client.AuthTestContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("slack-connector: failed to authenticate. Error: %w", err)
	}

	var enterpriseId string
	if res.EnterpriseID != "" {
		enterpriseId = res.EnterpriseID
		if enterpriseKey == "" {
			return nil, fmt.Errorf("slack-connector: enterprise account detected, but no enterprise token specified")
		}
	}
	enterpriseClient, err := enterprise.NewClient(
		httpClient,
		enterpriseKey,
		apiKey,
		res.EnterpriseID,
		ssoEnabled,
	)
	if err != nil {
		return nil, fmt.Errorf("slack-connector: failed to create enterprise client. Error: %w", err)
	}

	return &Slack{
		client:           client,
		apiKey:           apiKey,
		enterpriseClient: enterpriseClient,
		enterpriseID:     enterpriseId,
		ssoEnabled:       ssoEnabled,
	}, nil
}

func (s *Slack) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncer {
	return []connectorbuilder.ResourceSyncer{
		userBuilder(s.client, s.enterpriseID, s.enterpriseClient),
		workspaceBuilder(s.client, s.enterpriseID, s.enterpriseClient),
		userGroupBuilder(s.client, s.enterpriseID, s.enterpriseClient),
		workspaceRoleBuilder(s.client, s.enterpriseClient),
		enterpriseRoleBuilder(s.enterpriseID, s.enterpriseClient),
		groupBuilder(s.enterpriseClient, s.enterpriseID, s.ssoEnabled),
	}
}
