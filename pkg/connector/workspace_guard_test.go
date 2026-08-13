package connector

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/slack-go/slack"
)

// workspaceRoleGrants is the only part of Grants that is gated on the sync
// filter, so everything it returns must target workspace_role. If a grant of
// another type ever leaks in here, filtering workspace_role out of a sync would
// silently drop that grant too — which is exactly the bug the split avoids.
func TestWorkspaceRoleGrants_OnlyTargetsWorkspaceRole(t *testing.T) {
	workspaceID := &v2.ResourceId{ResourceType: resourceTypeWorkspace.Id, Resource: "T123"}
	userID := &v2.ResourceId{ResourceType: resourceTypeUser.Id, Resource: "U123"}

	// Flags chosen to light up several role branches at once.
	user := client.User{ID: "U123", IsOwner: true, IsAdmin: true}

	grants, err := workspaceRoleGrants(context.Background(), user, workspaceID, userID)
	if err != nil {
		t.Fatalf("workspaceRoleGrants: %v", err)
	}
	if len(grants) == 0 {
		t.Fatal("expected role grants for an owner/admin user")
	}
	for _, g := range grants {
		got := g.GetEntitlement().GetResource().GetId().GetResourceType()
		if got != resourceTypeWorkspaceRole.Id {
			t.Errorf("grant targets %q, want %q", got, resourceTypeWorkspaceRole.Id)
		}
	}
}

// newStubbedWorkspaceBuilder returns a builder whose user source is an
// httptest server serving users.list, so Grants can be exercised end to end.
func newStubbedWorkspaceBuilder(t *testing.T, skipWorkspaceRoleResourceType bool) *workspaceResourceType {
	t.Helper()

	const usersListResponse = `{
		"ok": true,
		"members": [
			{"id": "U123", "is_owner": true, "is_admin": true},
			{"id": "U456"},
			{"id": "U789", "is_stranger": true}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, usersListResponse)
	}))
	t.Cleanup(server.Close)

	slackClient := slack.New("test-token", slack.OptionAPIURL(server.URL+"/"))
	return workspaceBuilder(slackClient, nil, skipWorkspaceRoleResourceType)
}

// grantsByEntitlementType counts the grants Grants returned, keyed by the
// resource type of the entitlement they target.
func grantsByEntitlementType(grants []*v2.Grant) map[string]int {
	counts := map[string]int{}
	for _, g := range grants {
		counts[g.GetEntitlement().GetResource().GetId().GetResourceType()]++
	}
	return counts
}

// The regression this PR fixed: gating the whole function dropped the
// workspace's own member grants along with the role grants, which downstream
// reads as mass revocation. Drive Grants at both settings of the flag.
func TestWorkspaceGrants_MemberGrantsSurviveRoleFilter(t *testing.T) {
	ctx := context.Background()

	workspace, err := resources.NewResource("acme", resourceTypeWorkspace, "T123")
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	// Role in scope: both member grants and role grants.
	inScope, _, err := newStubbedWorkspaceBuilder(t, false).Grants(ctx, workspace, resources.SyncOpAttrs{})
	if err != nil {
		t.Fatalf("Grants with workspace_role in scope: %v", err)
	}
	inScopeCounts := grantsByEntitlementType(inScope)
	// U123 and U456 are members; U789 is a stranger and must be skipped.
	if got := inScopeCounts[resourceTypeWorkspace.Id]; got != 2 {
		t.Errorf("workspace member grants = %d, want 2", got)
	}
	if inScopeCounts[resourceTypeWorkspaceRole.Id] == 0 {
		t.Error("expected workspace_role grants for an owner/admin user")
	}

	// Role filtered out: the role grants go away, the member grants must not.
	filtered, _, err := newStubbedWorkspaceBuilder(t, true).Grants(ctx, workspace, resources.SyncOpAttrs{})
	if err != nil {
		t.Fatalf("Grants with workspace_role filtered: %v", err)
	}
	filteredCounts := grantsByEntitlementType(filtered)
	if got := filteredCounts[resourceTypeWorkspace.Id]; got != 2 {
		t.Errorf("workspace member grants = %d, want 2: filtering workspace_role must not drop membership", got)
	}
	if got := filteredCounts[resourceTypeWorkspaceRole.Id]; got != 0 {
		t.Errorf("workspace_role grants = %d, want 0", got)
	}
}

// The workspace type keeps its own member entitlement and member grants, so
// neither the resource-type skip annotations nor a resource-level SkipGrants may
// be applied to it — they suppress the whole pass, not just the role grants.
func TestWorkspaceResourceType_NoSkipAnnotations(t *testing.T) {
	rt := workspaceBuilder(nil, nil, true).ResourceType(context.Background())
	for _, a := range rt.GetAnnotations() {
		if a.MessageIs(&v2.SkipEntitlements{}) ||
			a.MessageIs(&v2.SkipEntitlementsAndGrants{}) ||
			a.MessageIs(&v2.SkipGrants{}) {
			t.Fatal("workspace must not carry skip annotations: it owns a member entitlement")
		}
	}
}
