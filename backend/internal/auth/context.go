// Package auth provides the auth-domain middleware (JWT
// verification) and the huma-side helpers (UserFromContext,
// ContextMiddleware, Permission, GinContextMiddleware) used by
// every huma handler.
package auth

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"

	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// ctxKey is the request-scope key for the authenticated user and
// the recovered gin context. Different named types so the
// struct values compare unequal and the two slots never
// collide in context.WithValue lookups.
type ctxKey struct{}

type userCtxKeyType struct{}
type ginCtxKeyType struct{}

var (
	userKey       = userCtxKeyType{}
	ginContextKey = ginCtxKeyType{}
)

// UserFromContext returns the authenticated user, or a zero-value
// User with ok=false when the request is anonymous.
func UserFromContext(ctx context.Context) (model.User, bool) {
	v, ok := ctx.Value(userKey).(model.User)
	return v, ok
}

// UserIDFromContext is a convenience for handlers that only need
// the user ID.
func UserIDFromContext(ctx context.Context) uint {
	u, ok := UserFromContext(ctx)
	if !ok {
		return 0
	}
	return u.ID
}

// UserRoleFromContext returns the role string for the authenticated
// user, or "" for anonymous requests.
func UserRoleFromContext(ctx context.Context) string {
	u, ok := UserFromContext(ctx)
	if !ok {
		return ""
	}
	return u.Role
}

// GinContextFromContext recovers the *gin.Context stashed by
// GinContextMiddleware. Use it from handlers that still need gin
// idioms (multipart uploads, c.PostForm, c.ClientIP). Returns
// nil when the middleware has not run.
func GinContextFromContext(ctx context.Context) *gin.Context {
	v, ok := ctx.Value(ginContextKey).(*gin.Context)
	if !ok {
		return nil
	}
	return v
}

// ContextMiddleware copies the user from the underlying gin context
// (set by middleware.JWTAuth and friends) into the request's
// context.Context so huma handlers can read it via
// UserFromContext. It must be registered on the huma.API via
// api.UseMiddleware. humagin runs gin middleware first, so by the
// time this fires the gin context has the user (or doesn't, for
// anonymous requests).
func ContextMiddleware(ctx huma.Context, next func(huma.Context)) {
	if g := humagin.Unwrap(ctx); g != nil {
		if u, ok := middleware.CurrentUser(g); ok {
			next(huma.WithContext(ctx, context.WithValue(ctx.Context(), userKey, u)))
			return
		}
	}
	next(ctx)
}

// GinContextMiddleware stashes the *gin.Context in the request
// context so handlers can recover it via GinContextFromContext.
// Use only for handlers that depend on gin idioms (e.g. multipart
// file uploads). Most handlers should not need it.
func GinContextMiddleware(ctx huma.Context, next func(huma.Context)) {
	if g := humagin.Unwrap(ctx); g != nil {
		newCtx := context.WithValue(ctx.Context(), ginContextKey, g)
		next(huma.WithContext(ctx, newCtx))
		return
	}
	next(ctx)
}

// Permission is the huma-side role check. It must be chained
// after ContextMiddleware and after JWTAuth has populated the
// gin context. It mirrors the gin `middleware.Auth.Permission`:
// an unauthenticated request gets 401 (the gin JWTAuth
// middleware would have rejected it already, so the
// permission middleware never sees it on gin routes; huma
// versions re-introduce the 401 here to keep the surface
// consistent for tests and clients that rely on the status
// code), and a logged-in user without the required role gets
// 403.
func Permission(requiredRoles ...string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		role := UserRoleFromContext(ctx.Context())
		if role == "" {
			ctx.SetStatus(http.StatusUnauthorized)
			_, _ = ctx.BodyWriter().Write([]byte(`{"error":"Authentication required"}`))
			return
		}
		if !hasRole(role, requiredRoles) {
			ctx.SetStatus(http.StatusForbidden)
			_, _ = ctx.BodyWriter().Write([]byte(`{"error":"Insufficient permissions"}`))
			return
		}
		next(ctx)
	}
}

// hasRole reports whether the given role is one of the required
// roles, or is the super admin role (which is implicitly allowed
// everywhere).
func hasRole(role string, required []string) bool {
	if role == model.RoleSuperAdmin {
		return true
	}
	for _, r := range required {
		if r == role {
			return true
		}
	}
	return false
}

