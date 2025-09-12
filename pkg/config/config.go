package config

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	AccessTokenField = field.StringField(
		"token",
		field.WithDisplayName("Access Token"),
		field.WithDescription("The Slack bot user oauth token used to connect to the Slack API"),
		field.WithRequired(true),
	)
	EnterpriseTokenField = field.StringField(
		"enterprise-token",
		field.WithDisplayName("Enterprise Token"),
		field.WithDescription("The Slack user oauth token used to connect to the Slack Enterprise Grid Admin API"),
	)
	SSOEnabledField = field.BoolField(
		"sso-enabled",
		field.WithDisplayName("SSO Enabled"),
		field.WithDescription("Flag indicating that the SSO has been configured for Enterprise Grid Organization. Enables usage of SCIM API"),
		field.WithDefaultValue(false),
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
		EnterpriseTokenField,
		SSOEnabledField,
		GovEnvironmentField,
	}

	// FieldRelationships defines relationships between the fields listed in
	// ConfigurationFields that can be automatically validated.
	// Every Gov Slack instance is an Enterprise Grid instance.
	FieldRelationships = []field.SchemaFieldRelationship{
		field.FieldsDependentOn(
			[]field.SchemaField{GovEnvironmentField},
			[]field.SchemaField{EnterpriseTokenField},
		),
	}

	Configuration = field.NewConfiguration(ConfigurationFields, field.WithConstraints(FieldRelationships...))
)
