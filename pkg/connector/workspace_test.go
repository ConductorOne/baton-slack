package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/require"
)

// fakeUsersListServer starts an httptest.Server that fakes Slack's
// users.list endpoint (see vendor/github.com/slack-go/slack/users.go for the
// expected response shape). It always returns a single page (empty
// next_cursor) containing one plain member and one workspace owner, so tests
// can assert on whether a role grant is emitted for the owner.
func fakeUsersListServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/users.list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"ok": true,
			"members": []map[string]interface{}{
				{
					"id":       "U_MEMBER",
					"team_id":  "T123",
					"name":     "regular-member",
					"deleted":  false,
					"is_owner": false,
					"is_bot":   false,
				},
				{
					"id":       "U_OWNER",
					"team_id":  "T123",
					"name":     "workspace-owner",
					"deleted":  false,
					"is_owner": true,
					"is_bot":   false,
				},
			},
			"response_metadata": map[string]interface{}{
				"next_cursor": "",
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// newTestWorkspaceGrants builds a workspaceResourceType wired to a fake
// users.list server (businessPlusClient left nil, so Grants exercises the
// o.client.GetUsersContext fallback path) and returns the grants produced
// for a single test workspace.
func newTestWorkspaceGrants(t *testing.T, syncWorkspaceRoles bool) []*grantResult {
	t.Helper()

	server := fakeUsersListServer(t)
	slackClient := slack.New("test-token", slack.OptionAPIURL(server.URL+"/"))

	o := workspaceBuilder(slackClient, nil, syncWorkspaceRoles)

	workspaceResource, err := resources.NewGroupResource(
		"Test Workspace",
		resourceTypeWorkspace,
		"T123",
		nil,
	)
	require.NoError(t, err)

	grants, _, err := o.Grants(context.Background(), workspaceResource, resources.SyncOpAttrs{})
	require.NoError(t, err)

	out := make([]*grantResult, 0, len(grants))
	for _, g := range grants {
		out = append(out, &grantResult{
			entitlementResourceType: g.GetEntitlement().GetResource().GetId().GetResourceType(),
		})
	}
	return out
}

type grantResult struct {
	entitlementResourceType string
}

func TestWorkspaceGrants_RoleSyncEnabled(t *testing.T) {
	grants := newTestWorkspaceGrants(t, true)

	memberGrants := 0
	roleGrants := 0
	for _, g := range grants {
		switch g.entitlementResourceType {
		case resourceTypeWorkspace.Id:
			memberGrants++
		case resourceTypeWorkspaceRole.Id:
			roleGrants++
		}
	}

	require.Greater(t, memberGrants, 0, "expected at least one member grant")
	require.Greater(t, roleGrants, 0, "expected at least one workspaceRole grant when syncWorkspaceRoles=true")
}

func TestWorkspaceGrants_RoleSyncDisabled(t *testing.T) {
	grants := newTestWorkspaceGrants(t, false)

	memberGrants := 0
	roleGrants := 0
	for _, g := range grants {
		switch g.entitlementResourceType {
		case resourceTypeWorkspace.Id:
			memberGrants++
		case resourceTypeWorkspaceRole.Id:
			roleGrants++
		}
	}

	require.Greater(t, memberGrants, 0, "expected member grants to remain unconditional")
	require.Equal(t, 0, roleGrants, "expected zero workspaceRole grants when syncWorkspaceRoles=false")
}
