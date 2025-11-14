package config

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

// Config represents the configuration for the Slack connector.
type Config struct {
	Token          string `mapstructure:"token"`
	EnterpriseToken string `mapstructure:"enterprise-token"`
	SSOEnabled     bool   `mapstructure:"sso-enabled"`
	GovEnvironment bool   `mapstructure:"gov-env"`
}

// GetString returns the string value for the given field name.
func (c *Config) GetString(fieldName string) string {
	switch fieldName {
	case "token":
		return c.Token
	case "enterprise-token":
		return c.EnterpriseToken
	default:
		return ""
	}
}

// GetBool returns the boolean value for the given field name.
func (c *Config) GetBool(fieldName string) bool {
	switch fieldName {
	case "sso-enabled":
		return c.SSOEnabled
	case "gov-env":
		return c.GovEnvironment
	default:
		return false
	}
}

// GetInt returns the integer value for the given field name.
func (c *Config) GetInt(fieldName string) int {
	return 0
}

// GetStringSlice returns the string slice value for the given field name.
func (c *Config) GetStringSlice(fieldName string) []string {
	return nil
}

// GetStringMap returns the string map value for the given field name.
func (c *Config) GetStringMap(fieldName string) map[string]any {
	return nil
}

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

	Configuration = field.NewConfiguration(
		ConfigurationFields,
		field.WithConstraints(FieldRelationships...),
		field.WithConnectorDisplayName("Slack"),
		field.WithHelpUrl("/docs/baton/slack"),
		field.WithIconUrl("/static/app-icons/slack.svg"),
	)
)
