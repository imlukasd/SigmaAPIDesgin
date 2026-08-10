package tenancy

type Permission string

const (
	PermissionOrganizationRead   Permission = "organization:read"
	PermissionOrganizationManage Permission = "organization:manage"
	PermissionProductRead        Permission = "product:read"
	PermissionProductManage      Permission = "product:manage"
)

func roleAllowsPermission(role MembershipRole, permission Permission) bool {
	switch permission {
	case PermissionOrganizationRead, PermissionProductRead:
		return role == MembershipRoleOwner ||
			role == MembershipRoleAdmin ||
			role == MembershipRoleMember
	case PermissionOrganizationManage, PermissionProductManage:
		return role == MembershipRoleOwner ||
			role == MembershipRoleAdmin
	default:
		return false
	}
}
