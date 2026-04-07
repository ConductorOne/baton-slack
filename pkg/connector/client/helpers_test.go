package client

import (
	"testing"
	"time"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/slack-go/slack"
)

func TestIsRateLimited(t *testing.T) {
	t.Run("nil annotations", func(t *testing.T) {
		if IsRateLimited(nil) {
			t.Error("expected false for nil annotations")
		}
	})

	t.Run("empty annotations", func(t *testing.T) {
		var annos annotations.Annotations
		if IsRateLimited(&annos) {
			t.Error("expected false for empty annotations")
		}
	})

	t.Run("overlimit annotation", func(t *testing.T) {
		var annos annotations.Annotations
		annos.WithRateLimiting(rateLimitDescription(4 * time.Second))
		if !IsRateLimited(&annos) {
			t.Error("expected true for overlimit annotation")
		}
	})

	t.Run("ok status annotation", func(t *testing.T) {
		var annos annotations.Annotations
		annos.WithRateLimiting(&v2.RateLimitDescription{
			Status:    v2.RateLimitDescription_STATUS_OK,
			Remaining: 50,
		})
		if IsRateLimited(&annos) {
			t.Error("expected false for OK status annotation")
		}
	})
}

func TestWrapErrorSetsRateLimitAnnotation(t *testing.T) {
	t.Run("rate limit error populates annotations", func(t *testing.T) {
		var annos annotations.Annotations
		err := &slack.RateLimitedError{RetryAfter: 4 * time.Second}
		wrappedErr := WrapError(err, "test", &annos)
		if wrappedErr == nil {
			t.Fatal("expected non-nil error")
		}
		if !IsRateLimited(&annos) {
			t.Error("expected rate limit annotation after WrapError with RateLimitedError")
		}
	})

	t.Run("non-rate-limit error does not populate rate limit annotation", func(t *testing.T) {
		var annos annotations.Annotations
		err := slack.SlackErrorResponse{Err: "user_not_found"}
		wrappedErr := WrapError(err, "test", &annos)
		if wrappedErr == nil {
			t.Fatal("expected non-nil error")
		}
		if IsRateLimited(&annos) {
			t.Error("expected no rate limit annotation for user_not_found error")
		}
	})
}

func TestRateLimitOverride(t *testing.T) {
	rl := RateLimitOverride()
	if rl.Status != v2.RateLimitDescription_STATUS_OVERLIMIT {
		t.Errorf("expected STATUS_OVERLIMIT, got %v", rl.Status)
	}
	if rl.Remaining != 0 {
		t.Errorf("expected Remaining=0, got %d", rl.Remaining)
	}
	resetIn := time.Until(rl.ResetAt.AsTime())
	if resetIn < 55*time.Second || resetIn > 65*time.Second {
		t.Errorf("expected ResetAt ~60s from now, got %v", resetIn)
	}
}

func TestRateLimitOverrideReplacesExisting(t *testing.T) {
	// Simulate: WrapError sets a 4s annotation, then we override to 60s
	var annos annotations.Annotations
	annos.WithRateLimiting(rateLimitDescription(4 * time.Second))

	if !IsRateLimited(&annos) {
		t.Fatal("expected rate limited after initial annotation")
	}

	// Override
	annos.WithRateLimiting(RateLimitOverride())

	rl := &v2.RateLimitDescription{}
	ok, err := annos.Pick(rl)
	if err != nil || !ok {
		t.Fatal("expected to find rate limit annotation after override")
	}

	resetIn := time.Until(rl.ResetAt.AsTime())
	if resetIn < 55*time.Second {
		t.Errorf("expected override to set ResetAt ~60s from now, got %v", resetIn)
	}
}
