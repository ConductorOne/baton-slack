package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/cli"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	cfg "github.com/conductorone/baton-slack/pkg/config"
	slackClient "github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
)

type Slack struct {
	client           *slack.Client
	apiKey           string
	enterpriseClient *slackClient.Client
	enterpriseID     string
	ssoEnabled       bool
	govEnv           bool
}

const govSlackApiUrl = "https://api.slack-gov.com/api/"

// Metadata returns metadata about the connector.
func (c *Slack) Metadata(ctx context.Context) (*v2.ConnectorMetadata, error) {
	return &v2.ConnectorMetadata{
		DisplayName: "Slack",
		Description: "Connector syncing users, workspaces, user groups and workspace roles from Slack to Baton.",
		AccountCreationSchema: &v2.ConnectorAccountCreationSchema{
			FieldMap: map[string]*v2.ConnectorAccountCreationSchema_Field{
				"channel_ids": {
					DisplayName: "ChannelIDs",
					Required:    true,
					Description: "Channel IDs the user will be invited to",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "ChannelIDs",
					Order:       1,
				},
				"email": {
					DisplayName: "Email",
					Required:    true,
					Description: "This email will be used as the login for the user.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "Email",
					Order:       2,
				},
				"team_id": {
					DisplayName: "WorkspaceID",
					Required:    true,
					Description: "The workspaceID the user will be invited to",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "TeamID",
					Order:       3,
				},
			},
		},
	}, nil
}

// Validate hits the Slack API to validate that the authenticated user has needed permissions.
func (s *Slack) Validate(ctx context.Context) (annotations.Annotations, error) {
	res, err := s.client.AuthTestContext(ctx)
	if err != nil {
		return nil, slackClient.WrapSlackClientError(err, "authenticating")
	}

	user, err := s.client.GetUserInfoContext(ctx, res.UserID)
	if err != nil {
		return nil, slackClient.WrapSlackClientError(err, "retrieving authenticated user")
	}

	isValidUser := user.IsAdmin || user.IsOwner || user.IsPrimaryOwner || user.IsBot
	if !isValidUser {
		return nil, uhttp.WrapErrors(
			codes.PermissionDenied,
			"authenticated user is not an admin, owner, primary owner or a bot",
			fmt.Errorf("user lacks required permissions"),
		)
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

func NewSlack(ctx context.Context, apiKey, enterpriseKey string, ssoEnabled bool, govEnv bool) (*Slack, error) {
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
	if govEnv {
		opts = append(opts, slack.OptionAPIURL(govSlackApiUrl))
	}
	client := slack.New(apiKey, opts...)

	res, err := client.AuthTestContext(ctx)
	if err != nil {
		return nil, slackClient.WrapSlackClientError(err, "authenticating during initialization")
	}

	var enterpriseId string
	if res.EnterpriseID != "" {
		enterpriseId = res.EnterpriseID
		if enterpriseKey == "" {
			return nil, uhttp.WrapErrors(
				codes.InvalidArgument,
				"enterprise account detected, but no enterprise token specified",
				fmt.Errorf("missing enterprise token"),
			)
		}
	}
	enterpriseClient, err := slackClient.NewClient(
		httpClient,
		enterpriseKey,
		apiKey,
		res.EnterpriseID,
		ssoEnabled,
		govEnv,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create enterprise client. Error: %w", err)
	}
	return &Slack{
		client:           client,
		apiKey:           apiKey,
		enterpriseClient: enterpriseClient,
		enterpriseID:     enterpriseId,
		ssoEnabled:       ssoEnabled,
		govEnv:           govEnv,
	}, nil
}

func New(ctx context.Context, config *cfg.Slack, opts *cli.ConnectorOpts) (connectorbuilder.ConnectorBuilderV2, []connectorbuilder.Opt, error) {
	cb, err := NewSlack(
		ctx,
		config.Token,
		config.EnterpriseToken,
		config.SsoEnabled,
		config.GovEnv,
	)
	if err != nil {
		return nil, nil, err
	}

	builderOpts := []connectorbuilder.Opt{}
	return cb, builderOpts, nil
}

func (s *Slack) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncerV2 {
	return []connectorbuilder.ResourceSyncerV2{
		userBuilder(s.client, s.enterpriseID, s.enterpriseClient),
		workspaceBuilder(s.client, s.enterpriseID, s.enterpriseClient),
		userGroupBuilder(s.client, s.enterpriseID, s.enterpriseClient),
		workspaceRoleBuilder(s.client, s.enterpriseID, s.enterpriseClient),
		enterpriseRoleBuilder(s.enterpriseID, s.enterpriseClient),
		groupBuilder(s.enterpriseClient, s.enterpriseID, s.ssoEnabled, s.govEnv),
	}
}
