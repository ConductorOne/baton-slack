package connector

import (
	"context"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/slack-go/slack"
)

func TestUserResourceMFAStatus(t *testing.T) {
	falseValue := false
	trueValue := true
	testCases := []struct {
		name        string
		has2FA      *bool
		wantEnabled bool
	}{
		{name: "omitted", has2FA: nil, wantEnabled: false},
		{name: "false", has2FA: &falseValue, wantEnabled: false},
		{name: "true", has2FA: &trueValue, wantEnabled: true},
	}

	parentResourceID := &v2.ResourceId{ResourceType: resourceTypeWorkspace.Id, Resource: "T123"}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			user, err := userResource(context.Background(), &slack.User{
				ID:     "U123",
				Name:   "alice",
				Has2FA: tc.has2FA,
				Profile: slack.UserProfile{
					Email: "alice@example.com",
				},
			}, parentResourceID)
			if err != nil {
				t.Fatalf("userResource: %v", err)
			}

			trait, err := resources.GetUserTrait(user)
			if err != nil {
				t.Fatalf("GetUserTrait: %v", err)
			}
			status := trait.GetMfaStatus()
			if tc.wantEnabled {
				if status == nil || !status.GetMfaEnabled() {
					t.Fatal("expected MFA status to report enabled")
				}
				return
			}
			if status != nil {
				t.Fatalf("expected no MFA status, got enabled=%t", status.GetMfaEnabled())
			}
		})
	}
}
