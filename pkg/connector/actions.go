package connector

import (
	"context"
	"fmt"

	config_sdk "github.com/conductorone/baton-sdk/pb/c1/config/v1"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/actions"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	ActionDisableUser = "disable_user"
	ActionEnableUser  = "enable_user"
)

var (
	disableUserSchema = &v2.BatonActionSchema{
		Name:        ActionDisableUser,
		DisplayName: "Disable User",
		Description: "Deactivate a Slack user account by setting active to false via SCIM API",
		Arguments: []*config_sdk.Field{
			{
				Name:        "user_id",
				DisplayName: "User ID",
				Description: "The Slack user ID to disable",
				IsRequired:  true,
				Field:       &config_sdk.Field_StringField{},
			},
		},
		ReturnTypes: []*config_sdk.Field{
			{
				Name:        "success",
				DisplayName: "Success",
				Description: "Indicates if the operation was successful",
				Field:       &config_sdk.Field_BoolField{},
			},
			{
				Name:        "message",
				DisplayName: "Message",
				Description: "A descriptive message about the operation result",
				Field:       &config_sdk.Field_StringField{},
			},
			{
				Name:        "user_id",
				DisplayName: "User ID",
				Description: "The Slack user ID that was processed",
				Field:       &config_sdk.Field_StringField{},
			},
		},
		ActionType: []v2.ActionType{
			v2.ActionType_ACTION_TYPE_ACCOUNT_DISABLE,
		},
	}
	enableUserSchema = &v2.BatonActionSchema{
		Name:        ActionEnableUser,
		DisplayName: "Enable User",
		Description: "Activate a Slack user account by setting active to true via SCIM API",
		Arguments: []*config_sdk.Field{
			{
				Name:        "user_id",
				DisplayName: "User ID",
				Description: "The Slack user ID to enable",
				IsRequired:  true,
				Field:       &config_sdk.Field_StringField{},
			},
		},
		ReturnTypes: []*config_sdk.Field{
			{
				Name:        "success",
				DisplayName: "Success",
				Description: "Indicates if the operation was successful",
				Field:       &config_sdk.Field_BoolField{},
			},
			{
				Name:        "message",
				DisplayName: "Message",
				Description: "A descriptive message about the operation result",
				Field:       &config_sdk.Field_StringField{},
			},
			{
				Name:        "user_id",
				DisplayName: "User ID",
				Description: "The Slack user ID that was processed",
				Field:       &config_sdk.Field_StringField{},
			},
		},
		ActionType: []v2.ActionType{
			v2.ActionType_ACTION_TYPE_ACCOUNT_ENABLE,
		},
	}
)

func (s *Slack) RegisterActionManager(ctx context.Context) (connectorbuilder.CustomActionManager, error) {
	l := ctxzap.Extract(ctx)

	actionManager := actions.NewActionManager(ctx)

	err := actionManager.RegisterAction(ctx, ActionDisableUser, disableUserSchema, s.handleDisableUser)
	if err != nil {
		l.Error("failed to register disable_user action", zap.Error(err))
		return nil, err
	}
	l.Info("registered disable_user action")

	err = actionManager.RegisterAction(ctx, ActionEnableUser, enableUserSchema, s.handleEnableUser)
	if err != nil {
		l.Error("failed to register enable_user action", zap.Error(err))
		return nil, err
	}

	l.Info("registered enable_user action")
	return actionManager, nil
}

// handleDisableUser deactivates a Slack user by setting active to false via SCIM API.
func (s *Slack) handleDisableUser(
	ctx context.Context,
	args *structpb.Struct,
) (*structpb.Struct, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	userIDValue, ok := args.Fields["user_id"]
	if !ok {
		return nil, nil, fmt.Errorf("user_id parameter is required")
	}

	userID := userIDValue.GetStringValue()
	if userID == "" {
		return nil, nil, fmt.Errorf("user_id cannot be empty")
	}

	l.Debug("disabling user via SCIM", zap.String("user_id", userID))

	if s.businessPlusClient == nil {
		return nil, nil, fmt.Errorf("enterprise client not available - SCIM API requires Enterprise Grid")
	}

	ratelimitData, err := s.businessPlusClient.DisableUser(ctx, userID)
	if err != nil {
		l.Error("failed to disable user", zap.String("user_id", userID), zap.Error(err))
		return nil, nil, fmt.Errorf("failed to disable user %s: %w", userID, err)
	}

	outputAnnotations := annotations.New()
	if ratelimitData != nil {
		outputAnnotations.WithRateLimiting(ratelimitData)
	}

	l.Info("user disabled successfully", zap.String("user_id", userID))

	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"success": {Kind: &structpb.Value_BoolValue{BoolValue: true}},
			"message": {Kind: &structpb.Value_StringValue{StringValue: fmt.Sprintf("User %s disabled successfully", userID)}},
			"user_id": {Kind: &structpb.Value_StringValue{StringValue: userID}},
		},
	}, outputAnnotations, nil
}

// handleEnableUser activates a Slack user by setting active to true via SCIM API.
func (s *Slack) handleEnableUser(
	ctx context.Context,
	args *structpb.Struct,
) (*structpb.Struct, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	userIDValue, ok := args.Fields["user_id"]
	if !ok {
		return nil, nil, fmt.Errorf("user_id parameter is required")
	}

	userID := userIDValue.GetStringValue()
	if userID == "" {
		return nil, nil, fmt.Errorf("user_id cannot be empty")
	}

	l.Debug("enabling user via SCIM", zap.String("user_id", userID))

	if s.businessPlusClient == nil {
		return nil, nil, fmt.Errorf("enterprise client not available - SCIM API requires Enterprise Grid")
	}

	ratelimitData, err := s.businessPlusClient.EnableUser(ctx, userID)
	if err != nil {
		l.Error("failed to enable user", zap.String("user_id", userID), zap.Error(err))
		return nil, nil, fmt.Errorf("failed to enable user %s: %w", userID, err)
	}

	outputAnnotations := annotations.New()
	if ratelimitData != nil {
		outputAnnotations.WithRateLimiting(ratelimitData)
	}

	l.Info("user enabled successfully", zap.String("user_id", userID))

	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"success": {Kind: &structpb.Value_BoolValue{BoolValue: true}},
			"message": {Kind: &structpb.Value_StringValue{StringValue: fmt.Sprintf("User %s enabled successfully", userID)}},
			"user_id": {Kind: &structpb.Value_StringValue{StringValue: userID}},
		},
	}, outputAnnotations, nil
}
