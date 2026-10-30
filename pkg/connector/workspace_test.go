package connector

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	sessions "github.com/conductorone/baton-sdk/pkg/types/sessions"
	"github.com/slack-go/slack"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// memSessionStore is a minimal in-memory implementation of
// sessions.SessionStore. The SDK ships no exported in-memory store, and
// workspaceResourceType.List writes workspace names through
// SetWorkspaceNames unconditionally, so both workspace tests need a stub to
// back resources.SyncOpAttrs{Session: ...}.
type memSessionStore struct {
	mu     sync.Mutex
	values map[string][]byte
}

func newMemSessionStore() *memSessionStore { return &memSessionStore{values: map[string][]byte{}} }

func (m *memSessionStore) Get(_ context.Context, key string, _ ...sessions.SessionStoreOption) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.values[key]
	return v, ok, nil
}

func (m *memSessionStore) GetMany(_ context.Context, keys []string, _ ...sessions.SessionStoreOption) (map[string][]byte, []string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string][]byte{}
	for _, k := range keys {
		if v, ok := m.values[k]; ok {
			out[k] = v
		}
	}
	return out, []string{}, nil
}

func (m *memSessionStore) Set(_ context.Context, key string, value []byte, _ ...sessions.SessionStoreOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = value
	return nil
}

func (m *memSessionStore) SetMany(_ context.Context, values map[string][]byte, _ ...sessions.SessionStoreOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range values {
		m.values[k] = v
	}
	return nil
}

func (m *memSessionStore) Delete(_ context.Context, key string, _ ...sessions.SessionStoreOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, key)
	return nil
}

func (m *memSessionStore) Clear(_ context.Context, _ ...sessions.SessionStoreOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values = map[string][]byte{}
	return nil
}

func (m *memSessionStore) GetAll(_ context.Context, _ string, _ ...sessions.SessionStoreOption) (map[string][]byte, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string][]byte, len(m.values))
	for k, v := range m.values {
		out[k] = v
	}
	return out, "", nil
}

// newStubbedTeamsBuilder returns a workspace builder whose slack client talks
// to an httptest server serving the given auth.teams.list-shaped response,
// plus the session store the List call must be given.
func newStubbedTeamsBuilder(t *testing.T, response string) (*workspaceResourceType, *memSessionStore) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Pin the live discovery endpoint: List must keep calling Slack's
		// auth.teams.list (via slack-go ListTeamsContext), not any other method.
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/auth.teams.list") {
			http.Error(w, "unexpected stub request: "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(server.Close)
	store := newMemSessionStore()
	slackClient := slack.New("test-token", slack.OptionAPIURL(server.URL+"/"))
	return workspaceBuilder(slackClient, nil, false), store
}

// TestWorkspaceList_MultiTeamDiscovery pins the live workspace discovery path:
// the teams returned by auth.teams.list become workspace resources whose IDs
// derive from slack.Team.ID.
func TestWorkspaceList_MultiTeamDiscovery(t *testing.T) {
	const multiTeamResponse = `{"ok":true,"teams":[{"id":"T1","name":"One"},{"id":"T2","name":"Two"}],"response_metadata":{"next_cursor":""}}`
	ctx := context.Background()
	builder, store := newStubbedTeamsBuilder(t, multiTeamResponse)

	rr, result, err := builder.List(ctx, nil, resources.SyncOpAttrs{Session: store})
	if err != nil {
		t.Fatalf("List with two teams: %v", err)
	}
	if len(rr) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(rr))
	}
	gotIDs := []string{rr[0].GetId().GetResource(), rr[1].GetId().GetResource()}
	sort.Strings(gotIDs)
	wantIDs := []string{"T1", "T2"}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Errorf("resource ID = %q, want %q", gotIDs[i], wantIDs[i])
		}
	}
	for _, r := range rr {
		if r.GetId().GetResourceType() != resourceTypeWorkspace.Id {
			t.Errorf("resource %s: resource type = %q, want %q", r.GetId().GetResource(), r.GetId().GetResourceType(), resourceTypeWorkspace.Id)
		}
	}
	displayNames := map[string]string{}
	for _, r := range rr {
		displayNames[r.GetId().GetResource()] = r.GetDisplayName()
	}
	if displayNames["T1"] != "One" || displayNames["T2"] != "Two" {
		t.Errorf("display names = %v, want One/Two", displayNames)
	}
	if result == nil {
		t.Fatal("expected non-nil SyncOpResults")
	}
	if result.NextPageToken != "" {
		t.Errorf("next page token = %q, want empty (single page)", result.NextPageToken)
	}
}

// TestWorkspaceList_ErrorPropagation pins fail-closed behavior: an ok:false
// API response must surface as an error carrying the client.WrapError context
// string, with no resources and the annotation-bearing results object.
func TestWorkspaceList_ErrorPropagation(t *testing.T) {
	ctx := context.Background()
	builder, store := newStubbedTeamsBuilder(t, `{"ok":false,"error":"invalid_auth"}`)

	rr, result, err := builder.List(ctx, nil, resources.SyncOpAttrs{Session: store})
	if err == nil {
		t.Fatal("expected error for ok:false response, got nil")
	}
	if len(rr) != 0 {
		t.Errorf("expected no resources on error path, got %d", len(rr))
	}
	if result == nil {
		t.Fatal("expected non-nil SyncOpResults on error path")
	}
	// Note: for a plain ok:false error (not rate limiting / HTTP status), the
	// existing WrapError path returns the results object with an empty
	// Annotations slice; pin the non-nil results shape, not a populated one.
	if !strings.Contains(err.Error(), "error listing teams") {
		t.Errorf("error message %q does not contain %q", err.Error(), "error listing teams")
	}
	// The error must keep its typed gRPC classification (invalid_auth maps to
	// codes.Unauthenticated in client.WrapError) so the SDK's retry/rate-limit
	// plumbing can still classify it.
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Errorf("WrapError classification lost: got %v, want %v (invalid_auth must map to Unauthenticated)", got, codes.Unauthenticated)
	}
	// Pin the empty-annotations invariant for a plain ok:false (non-rate-limited)
	// error: no rate-limit annotation may leak into the results here.
	if n := len(result.Annotations); n != 0 {
		t.Errorf("expected 0 annotations for plain ok:false error, got %d", n)
	}
}
