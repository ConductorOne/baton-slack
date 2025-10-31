package connector

import (
	"context"

	"github.com/conductorone/baton-sdk/pkg/annotations"
	enterprise "github.com/conductorone/baton-slack/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

// populateAdminUsersCache fetches all admin users and populates the cache.
// This is used to enrich SCIM users with SSO, 2FA, and bot status information.
func (o *userResourceType) populateAdminUsersCache(ctx context.Context) (annotations.Annotations, error) {
	o.adminCacheMutex.RLock()
	if o.adminUsersCache != nil {
		o.adminCacheMutex.RUnlock()
		return nil, nil
	}
	o.adminCacheMutex.RUnlock()

	l := ctxzap.Extract(ctx)
	l.Info("Populating admin users cache for SCIM enrichment")

	var annos annotations.Annotations
	o.adminCacheMutex.Lock()
	defer o.adminCacheMutex.Unlock()

	if o.adminUsersCache != nil {
		return nil, nil
	}

	o.adminUsersCache = make(map[string]enterprise.UserAdmin)
	cursor := ""
	for {
		adminUsers, nextCursor, adminRatelimit, err := o.enterpriseClient.GetUsersAdmin(ctx, cursor)
		if adminRatelimit != nil {
			annos.WithRateLimiting(adminRatelimit)
		}
		if err != nil {
			l.Warn("failed to fetch admin users for enrichment, continuing without SSO/2FA/bot data", zap.Error(err))
			return annos, err
		}
		for _, adminUser := range adminUsers {
			o.adminUsersCache[adminUser.ID] = adminUser
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	l.Info("Admin users cache populated", zap.Int("count", len(o.adminUsersCache)))
	return annos, nil
}

// getAdminUser retrieves an admin user from the cache by user ID.
// Populates the cache on first access if not already populated.
// Returns the admin user and a boolean indicating if it was found.
func (o *userResourceType) getAdminUser(ctx context.Context, userID string) (*enterprise.UserAdmin, bool) {
	o.adminCacheMutex.RLock()
	if o.adminUsersCache != nil {
		adminUser, ok := o.adminUsersCache[userID]
		o.adminCacheMutex.RUnlock()
		if ok {
			return &adminUser, true
		}
		return nil, false
	}
	o.adminCacheMutex.RUnlock()

	_, err := o.populateAdminUsersCache(ctx)
	if err != nil {
		return nil, false
	}

	o.adminCacheMutex.RLock()
	defer o.adminCacheMutex.RUnlock()

	if o.adminUsersCache == nil {
		return nil, false
	}

	adminUser, ok := o.adminUsersCache[userID]
	if !ok {
		return nil, false
	}

	return &adminUser, true
}
