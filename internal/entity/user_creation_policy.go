package entity

// UserCreationPolicy tells the user get-or-create flow whether the calling
// context authorizes creating a user that does not exist yet, and which extra
// roles the created user receives.
type UserCreationPolicy struct {
	// AllowCreate — creation is authorized by the call itself (invitation
	// accept, dev test-roles header). When false, creation is allowed only by
	// open signup.
	AllowCreate bool
	// GrantRoles are roles assigned on top of DefaultRoles (dev test-roles
	// header only; invitation roles are assigned by the accept flow in its own
	// transaction).
	GrantRoles []Role
}
