package main

import (
	"context"
	"fmt"
	"os"

	configSdk "github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/types"
	"github.com/conductorone/baton-slack/pkg/config"
	"github.com/conductorone/baton-slack/pkg/connector"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	connectorName = "baton-slack"
	version       = "dev"
)

func main() {
	ctx := context.Background()

	_, cmd, err := configSdk.DefineConfiguration(
		ctx,
		connectorName,
		getConnector,
		config.Configuration,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	cmd.Version = version

	err = cmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func getConnector(ctx context.Context, v *viper.Viper) (types.ConnectorServer, error) {
	logger := ctxzap.Extract(ctx)
	cb, err := connector.New(
		ctx,
		v.GetString(config.AccessTokenField.FieldName),
		v.GetString(config.EnterpriseTokenField.FieldName),
		v.GetBool(config.SSOEnabledField.FieldName),
		v.GetBool(config.GovEnvironmentField.FieldName),
	)
	if err != nil {
		logger.Error("error creating connector", zap.Error(err))
		return nil, err
	}

	c, err := connectorbuilder.NewConnector(ctx, cb)
	if err != nil {
		logger.Error("error creating connector", zap.Error(err))
		return nil, err
	}

	return c, nil
}
