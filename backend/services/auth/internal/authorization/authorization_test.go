package authorization

import (
	"context"
	"testing"

	"auth/internal/repository"
)

type stubUserRepository struct {
	user  *repository.User
	roles []string
}

func (s *stubUserRepository) FindByID(ctx context.Context, userID string) (*repository.User, error) {
	if s.user == nil {
		return nil, repository.ErrUserNotFound
	}
	return s.user, nil
}

func (s *stubUserRepository) GetRoles(ctx context.Context, userID string) ([]string, error) {
	return s.roles, nil
}

type stubRoleRepository struct {
	roles map[string]*repository.Role
}

func (s *stubRoleRepository) FindByName(ctx context.Context, name string) (*repository.Role, error) {
	if role, ok := s.roles[name]; ok {
		return role, nil
	}
	return nil, repository.ErrRoleNotFound
}

type stubRolePermissionRepository struct {
	permissions map[string][]*repository.Permission
}

func (s *stubRolePermissionRepository) GetPermissionsByRoleID(ctx context.Context, roleID string) ([]*repository.Permission, error) {
	return s.permissions[roleID], nil
}

func TestAuthorizationService_HasPermissionAllow(t *testing.T) {
	ctx := context.Background()
	userRepo := &stubUserRepository{
		user:  &repository.User{ID: "user-1", Estado: true},
		roles: []string{"ADMIN"},
	}
	roleRepo := &stubRoleRepository{roles: map[string]*repository.Role{
		"ADMIN": {ID: "role-admin", Name: "ADMIN"},
	}}
	permRepo := &stubRolePermissionRepository{permissions: map[string][]*repository.Permission{
		"role-admin": {
			{Codigo: "users.read", Estado: true},
			{Codigo: "expedientes.read", Estado: true},
		},
	}}
	service := NewAuthorizationService(userRepo, roleRepo, permRepo)

	ok, err := service.HasPermission(ctx, "user-1", "users.read")
	if err != nil || !ok {
		t.Fatalf("expected users.read to be allowed, got ok=%v err=%v", ok, err)
	}
}

func TestAuthorizationService_HasPermissionDeny(t *testing.T) {
	ctx := context.Background()
	userRepo := &stubUserRepository{
		user:  &repository.User{ID: "user-2", Estado: true},
		roles: []string{"SOLICITANTE"},
	}
	roleRepo := &stubRoleRepository{roles: map[string]*repository.Role{
		"SOLICITANTE": {ID: "role-solicitante", Name: "SOLICITANTE"},
	}}
	permRepo := &stubRolePermissionRepository{permissions: map[string][]*repository.Permission{
		"role-solicitante": {
			{Codigo: "expedientes.read_own", Estado: true},
		},
	}}
	service := NewAuthorizationService(userRepo, roleRepo, permRepo)

	ok, err := service.HasPermission(ctx, "user-2", "users.delete")
	if err != nil || ok {
		t.Fatalf("expected users.delete to be denied, got ok=%v err=%v", ok, err)
	}
}

func TestAuthorizationService_UsesMultipleRoles(t *testing.T) {
	ctx := context.Background()
	userRepo := &stubUserRepository{
		user:  &repository.User{ID: "user-3", Estado: true},
		roles: []string{"ADMIN", "SOLICITANTE"},
	}
	roleRepo := &stubRoleRepository{roles: map[string]*repository.Role{
		"ADMIN":       {ID: "role-admin", Name: "ADMIN"},
		"SOLICITANTE": {ID: "role-solicitante", Name: "SOLICITANTE"},
	}}
	permRepo := &stubRolePermissionRepository{permissions: map[string][]*repository.Permission{
		"role-admin": {
			{Codigo: "users.read", Estado: true},
		},
		"role-solicitante": {
			{Codigo: "expedientes.read_own", Estado: true},
		},
	}}
	service := NewAuthorizationService(userRepo, roleRepo, permRepo)

	ok, err := service.HasPermission(ctx, "user-3", "users.read")
	if err != nil || !ok {
		t.Fatalf("expected ADMIN permission to be allowed, got ok=%v err=%v", ok, err)
	}

	ok, err = service.HasPermission(ctx, "user-3", "expedientes.read_own")
	if err != nil || !ok {
		t.Fatalf("expected SOLICITANTE permission to be allowed, got ok=%v err=%v", ok, err)
	}
}

func TestAuthorizationService_UserInactive(t *testing.T) {
	ctx := context.Background()
	userRepo := &stubUserRepository{
		user:  &repository.User{ID: "user-4", Estado: false},
		roles: []string{"ADMIN"},
	}
	roleRepo := &stubRoleRepository{roles: map[string]*repository.Role{
		"ADMIN": {ID: "role-admin", Name: "ADMIN"},
	}}
	permRepo := &stubRolePermissionRepository{permissions: map[string][]*repository.Permission{
		"role-admin": {{Codigo: "users.read", Estado: true}},
	}}
	service := NewAuthorizationService(userRepo, roleRepo, permRepo)

	ok, err := service.HasPermission(ctx, "user-4", "users.read")
	if err != nil || ok {
		t.Fatalf("expected inactive user to be denied, got ok=%v err=%v", ok, err)
	}
}

func TestAuthorizationService_IgnoresInactivePermission(t *testing.T) {
	ctx := context.Background()
	userRepo := &stubUserRepository{
		user:  &repository.User{ID: "user-5", Estado: true},
		roles: []string{"ADMIN"},
	}
	roleRepo := &stubRoleRepository{roles: map[string]*repository.Role{
		"ADMIN": {ID: "role-admin", Name: "ADMIN"},
	}}
	permRepo := &stubRolePermissionRepository{permissions: map[string][]*repository.Permission{
		"role-admin": {{Codigo: "users.read", Estado: false}},
	}}
	service := NewAuthorizationService(userRepo, roleRepo, permRepo)

	ok, err := service.HasPermission(ctx, "user-5", "users.read")
	if err != nil || ok {
		t.Fatalf("expected inactive permission to be denied, got ok=%v err=%v", ok, err)
	}
}

func TestAuthorizationService_GetPermissionsByUserID(t *testing.T) {
	ctx := context.Background()
	userRepo := &stubUserRepository{
		user:  &repository.User{ID: "user-6", Estado: true},
		roles: []string{"ADMIN", "SOLICITANTE"},
	}
	roleRepo := &stubRoleRepository{roles: map[string]*repository.Role{
		"ADMIN":       {ID: "role-admin", Name: "ADMIN"},
		"SOLICITANTE": {ID: "role-solicitante", Name: "SOLICITANTE"},
	}}
	permRepo := &stubRolePermissionRepository{permissions: map[string][]*repository.Permission{
		"role-admin": { {Codigo: "users.read", Estado: true}, {Codigo: "expedientes.read", Estado: true} },
		"role-solicitante": { {Codigo: "expedientes.read_own", Estado: true} },
	}}
	service := NewAuthorizationService(userRepo, roleRepo, permRepo)

	permissions, err := service.GetPermissionsByUserID(ctx, "user-6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(permissions) != 3 {
		t.Fatalf("expected 3 permissions, got %d: %v", len(permissions), permissions)
	}
}
