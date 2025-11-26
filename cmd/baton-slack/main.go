package main

import (
	"context"

	"github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorrunner"
	cfg "github.com/conductorone/baton-slack/pkg/config"
	"github.com/conductorone/baton-slack/pkg/connector"
)

var (
	connectorName = "baton-slack"
	version       = "dev"
)

func main() {
	ctx := context.Background()
	config.RunConnector(ctx, connectorName, version, cfg.Configuration, connector.New,
		connectorrunner.WithSessionStoreEnabled())
}
