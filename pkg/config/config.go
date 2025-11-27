package config

//go:generate go run ./gen

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	AccessTokenField = field.StringField(
		"token",
		field.WithDisplayName("Access Token"),
		field.WithDescription("The Slack bot user oauth token used to connect to the Slack API"),
		field.WithRequired(true),
		field.WithIsSecret(true),
	)

	// ConfigurationFields defines the external configuration required for the
	// connector to run. Note: these fields can be marked as optional or
	// required.
	ConfigurationFields = []field.SchemaField{
		AccessTokenField,
	}

	Configuration = field.NewConfiguration(
		ConfigurationFields,
		field.WithConnectorDisplayName("Slack"),
		field.WithHelpUrl("/docs/baton/slack"),
		field.WithIconUrl("/static/app-icons/slack.svg"),
	)
)
