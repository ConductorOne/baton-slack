package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/slack-go/slack"
)

type mfaTestCase struct {
	name        string
	has2FA      *bool
	wantEnabled bool
}

func mfaTestCases() []mfaTestCase {
	falseValue := false
	trueValue := true
	return []mfaTestCase{
		{name: "omitted", has2FA: nil, wantEnabled: false},
		{name: "false", has2FA: &falseValue, wantEnabled: false},
		{name: "true", has2FA: &trueValue, wantEnabled: true},
	}
}

func assertMFAStatus(t *testing.T, user *v2.Resource, wantEnabled bool) {
	t.Helper()

	trait, err := resources.GetUserTrait(user)
	if err != nil {
		t.Fatalf("GetUserTrait: %v", err)
	}

	status := trait.GetMfaStatus()
	if wantEnabled {
		if status == nil || !status.GetMfaEnabled() {
			t.Fatal("expected MFA status to report enabled")
		}
		return
	}
	if status != nil {
		t.Fatalf("expected no MFA status, got enabled=%t", status.GetMfaEnabled())
	}
}

func TestUserResourceMFAStatus(t *testing.T) {
	parentResourceID := &v2.ResourceId{ResourceType: resourceTypeWorkspace.Id, Resource: "T123"}

	for _, tc := range mfaTestCases() {
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

			assertMFAStatus(t, user, tc.wantEnabled)
		})
	}
}

func TestSCIMUserResourceMFAStatus(t *testing.T) {
	parentResourceID := &v2.ResourceId{ResourceType: resourceTypeWorkspace.Id, Resource: "T123"}

	for _, tc := range mfaTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			response, err := json.Marshal(struct {
				OK   bool       `json:"ok"`
				User slack.User `json:"user"`
			}{
				OK: true,
				User: slack.User{
					ID:     "U123",
					Name:   "alice",
					Has2FA: tc.has2FA,
					Profile: slack.UserProfile{
						Email: "alice@example.com",
					},
				},
			})
			if err != nil {
				t.Fatalf("marshal Slack response: %v", err)
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(response)
			}))
			t.Cleanup(server.Close)

			builder := &userResourceType{
				client: slack.New("test-token", slack.OptionAPIURL(server.URL+"/")),
			}
			user, err := builder.scimUserResource(
				context.Background(),
				client.UserResource{ID: "U123", Active: true},
				parentResourceID,
			)
			if err != nil {
				t.Fatalf("scimUserResource: %v", err)
			}

			assertMFAStatus(t, user, tc.wantEnabled)
		})
	}
}
