package tenancy

import "context"

type tenantContextKey struct{}

type TenantContext struct {
	Organization Organization
	Membership   Membership
	Permission   Permission
}

func ContextWithTenant(ctx context.Context, tenant TenantContext) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenant)
}

func TenantFromContext(ctx context.Context) (TenantContext, bool) {
	tenant, ok := ctx.Value(tenantContextKey{}).(TenantContext)
	return tenant, ok
}
