package main

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-sdk/pkg/cli"
	"github.com/spf13/cobra"
)

// config defines the external configuration required for the connector to run.
type config struct {
	cli.BaseConfig  `mapstructure:",squash"` // Puts the base config options in the same place as the connector options
	AccessToken     string                   `mapstructure:"token"`
	EnterpriseToken string                   `mapstructure:"enterprise-token"`
	SSOEnabled      bool                     `mapstructure:"sso-enabled"`
}

// validateConfig is run after the configuration is loaded, and should return an error if it isn't valid.
func validateConfig(ctx context.Context, cfg *config) error {
	if cfg.AccessToken == "" {
		return fmt.Errorf("access token is missing")
	}

	return nil
}

// cmdFlags sets the cmdFlags required for the connector.
func cmdFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String("token", "", "The Slack bot user oauth token used to connect to the Slack API. ($BATON_TOKEN)")
	cmd.PersistentFlags().String("enterprise-token", "", "The Slack user oauth token used to connect to the Slack Enterprise Grid Admin API. ($BATON_ENTERPRISE_TOKEN)")
	cmd.PersistentFlags().String("sso-enabled", "", "Flag indicating that the SSO has been configured for Enterprise Grid Organization. Enables usage of SCIM API. ($BATON_SSO_ENABLED)")
}
