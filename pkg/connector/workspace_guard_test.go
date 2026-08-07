package connector

import (
	"context"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-slack/pkg/connector/client"
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

// The workspace member grant is emitted by Grants itself, outside the gated
// helper, so it must not be one of the grants the helper produces.
func TestWorkspaceRoleGrants_ExcludesWorkspaceMemberGrant(t *testing.T) {
	workspaceID := &v2.ResourceId{ResourceType: resourceTypeWorkspace.Id, Resource: "T123"}
	userID := &v2.ResourceId{ResourceType: resourceTypeUser.Id, Resource: "U123"}

	workspace, err := resources.NewResource("acme", resourceTypeWorkspace, "T123")
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}
	memberGrant := grant.NewGrant(workspace, memberEntitlement, userID)

	grants, err := workspaceRoleGrants(context.Background(), client.User{ID: "U123"}, workspaceID, userID)
	if err != nil {
		t.Fatalf("workspaceRoleGrants: %v", err)
	}
	for _, g := range grants {
		if g.GetId() == memberGrant.GetId() {
			t.Fatal("workspace member grant must be emitted unconditionally, not from the gated role helper")
		}
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
