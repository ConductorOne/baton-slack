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
	BaseURLField = field.StringField(
		"base-url",
		field.WithDisplayName("Base URL"),
		field.WithDescription("Override the Slack API URL (for testing or enterprise deployments)"),
		field.WithHidden(true),
		field.WithExportTarget(field.ExportTargetCLIOnly),
	)
	BusinessPlusTokenField = field.StringField(
		"business-plus-token",
		field.WithDisplayName("Business Plus Token"),
		field.WithDescription("The Slack user oauth token used to connect to the Slack Admin API (Business+ or Enterprise Grid)"),
		field.WithIsSecret(true),
	)
	GovEnvironmentField = field.BoolField(
		"gov-env",
		field.WithDisplayName("Gov Environment"),
		field.WithDescription("Flag indicating to use Slack-Gov environment."),
		field.WithDefaultValue(false),
	)

	// ConfigurationFields defines the external configuration required for the
	// connector to run. Note: these fields can be marked as optional or
	// required.
	ConfigurationFields = []field.SchemaField{
		AccessTokenField,
		BusinessPlusTokenField,
		GovEnvironmentField,
		BaseURLField,
	}

	// FieldRelationships defines relationships between the fields listed in
	// ConfigurationFields that can be automatically validated.
	// Every Gov Slack instance is a Business+ or Enterprise Grid instance.
	FieldRelationships = []field.SchemaFieldRelationship{
		field.FieldsDependentOn(
			[]field.SchemaField{GovEnvironmentField},
			[]field.SchemaField{BusinessPlusTokenField},
		),
	}

	Configuration = field.NewConfiguration(
		ConfigurationFields,
		field.WithConnectorDisplayName("Slack"),
		field.WithHelpUrl("/docs/baton/slack"),
		field.WithIconUrl("/static/app-icons/slack.svg"),
		field.WithConstraints(FieldRelationships...),
	)
)
