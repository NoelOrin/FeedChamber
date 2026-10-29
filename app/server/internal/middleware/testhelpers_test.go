package middleware

// fakePerms is a PermissionChecker stub shared by this package's tests.
// It was previously defined in space_permission_test.go, which was removed
// together with the unmounted RequireSpacePermission middleware.
type fakePerms struct {
	globalAllow bool
	spaceAllow  map[string]bool
}

func (f *fakePerms) HasPermission(role, code string) bool { return f.globalAllow }

func (f *fakePerms) HasPermissionInSpace(role, spaceID, code string) bool {
	if v, ok := f.spaceAllow[spaceID]; ok {
		return v
	}
	return f.globalAllow
}
