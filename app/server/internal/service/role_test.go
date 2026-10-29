package service

import (
	"context"
	"testing"

	"orinforum/internal/model"
)

// MockRoleRepository is a mock implementation of RoleRepository for testing
type MockRoleRepository struct {
	roles              []model.Role
	usersWithRole      map[string]bool
	deletedPermissions []string
	deletedRoles       []string
}

func (m *MockRoleRepository) List(ctx context.Context) ([]model.Role, error) {
	return m.roles, nil
}

func (m *MockRoleRepository) GetByName(ctx context.Context, name string) (*model.Role, error) {
	for _, role := range m.roles {
		if role.Name == name {
			return &role, nil
		}
	}
	return nil, nil
}

func (m *MockRoleRepository) Create(ctx context.Context, role *model.Role) error {
	m.roles = append(m.roles, *role)
	return nil
}

func (m *MockRoleRepository) CreateIfNotExists(ctx context.Context, role *model.Role) error {
	for _, r := range m.roles {
		if r.Name == role.Name {
			return nil
		}
	}
	return m.Create(ctx, role)
}

func (m *MockRoleRepository) Delete(ctx context.Context, id string) error {
	m.deletedRoles = append(m.deletedRoles, id)
	return nil
}

func (m *MockRoleRepository) HasUsersWithRole(ctx context.Context, roleName string) (bool, error) {
	return m.usersWithRole[roleName], nil
}

func (m *MockRoleRepository) DeleteRolePermissions(ctx context.Context, roleName string) error {
	m.deletedPermissions = append(m.deletedPermissions, roleName)
	return nil
}

func (m *MockRoleRepository) Update(ctx context.Context, id string, newName string, displayName string, description string) error {
	for i, r := range m.roles {
		if r.ID == id {
			if newName != "" {
				m.roles[i].Name = newName
			}
			if displayName != "" {
				m.roles[i].DisplayName = displayName
			}
			if description != "" {
				m.roles[i].Description = description
			}
			break
		}
	}
	return nil
}

func TestDeleteRole_BuiltinRole(t *testing.T) {
	repo := &MockRoleRepository{
		roles: []model.Role{
			{ID: "1", Name: "admin"},
			{ID: "2", Name: "user"},
			{ID: "3", Name: "ban"},
		},
		usersWithRole: make(map[string]bool),
	}
	svc := NewRoleService(repo)

	tests := []struct {
		name    string
		roleID  string
		wantErr bool
	}{
		{"Delete admin role", "1", true},
		{"Delete user role", "2", true},
		{"Delete ban role", "3", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Delete(context.Background(), tt.roleID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeleteRole_RoleWithUsers(t *testing.T) {
	repo := &MockRoleRepository{
		roles: []model.Role{
			{ID: "4", Name: "moderator"},
		},
		usersWithRole: map[string]bool{
			"moderator": true,
		},
	}
	svc := NewRoleService(repo)

	err := svc.Delete(context.Background(), "4")
	if err == nil {
		t.Error("Delete() should return error when role has users")
	}
}

func TestDeleteRole_Success(t *testing.T) {
	repo := &MockRoleRepository{
		roles: []model.Role{
			{ID: "5", Name: "custom_role"},
		},
		usersWithRole:      make(map[string]bool),
		deletedPermissions: make([]string, 0),
		deletedRoles:       make([]string, 0),
	}
	svc := NewRoleService(repo)

	err := svc.Delete(context.Background(), "5")
	if err != nil {
		t.Errorf("Delete() unexpected error = %v", err)
	}

	// Verify role_permissions were deleted
	if len(repo.deletedPermissions) != 1 || repo.deletedPermissions[0] != "custom_role" {
		t.Error("Delete() should have deleted role permissions")
	}

	// Verify role was deleted
	if len(repo.deletedRoles) != 1 || repo.deletedRoles[0] != "5" {
		t.Error("Delete() should have deleted the role")
	}
}
