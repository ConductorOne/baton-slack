package connector

import (
	"context"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
)

// Every grant the workspace syncer emits targets workspace_role, so when that
// type is excluded from the sync it must emit nothing — and must not page
// through users to discover that.
func TestWorkspaceBuilder_Grants_SkipWorkspaceRole(t *testing.T) {
	// A nil client would panic if the guard failed to short-circuit.
	b := workspaceBuilder(nil, nil, true)

	res, err := resources.NewResource("acme", resourceTypeWorkspace, "T123")
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	grants, results, err := b.Grants(context.Background(), res, resources.SyncOpAttrs{})
	if err != nil {
		t.Fatalf("Grants: %v", err)
	}
	if len(grants) != 0 {
		t.Fatalf("expected no grants when workspace_role is filtered out, got %d", len(grants))
	}
	if results == nil {
		t.Fatal("expected non-nil SyncOpResults")
	}
}

// The workspace type keeps its own member entitlement, so the resource-type
// skip annotations must not be applied to it.
func TestWorkspaceResourceType_NoSkipAnnotations(t *testing.T) {
	rt := workspaceBuilder(nil, nil, true).ResourceType(context.Background())
	for _, a := range rt.GetAnnotations() {
		if a.MessageIs(&v2.SkipEntitlements{}) || a.MessageIs(&v2.SkipEntitlementsAndGrants{}) {
			t.Fatal("workspace must not carry skip annotations: it owns a member entitlement")
		}
	}
}
