// Package trace provides Sentry-based user context management.
package trace

import (
	"context"

	"github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean/v2/config"
)

// SetUser sets user information into Sentry's Scope: User Effect Analysis.
// Returns the context with Hub attached for propagation.
//
// Example:
//
//	ctx := trace.SetUser(c.Request().Context(), sentry.User{
//	    ID:        "12345",
//	    Email:     "user@example.com",
//	    IPAddress: c.RealIP(),
//	    Username:  "johndoe",
//	    Name:      "John Doe",
//		Data:      map[string]string{"customerID":"1"}
//	})
func SetUser(ctx context.Context, user sentry.User) context.Context {
	// Early return if Sentry is disabled
	if !config.Bean.Sentry.On {
		return ctx
	}

	// Get Hub from context, or clone from global if not present
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
		ctx = sentry.SetHubOnContext(ctx, hub)
	}

	// Set user info to Scope
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetUser(user)
	})

	return ctx
}
