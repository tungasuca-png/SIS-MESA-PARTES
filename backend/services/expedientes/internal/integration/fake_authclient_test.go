package integration

import (
	"context"

	"auth/authclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// scriptedAuthClient es un doble de Auth SOLO para las suites de REGLAS DE
// NEGOCIO de expedientes (área/ownership/estado/etc.), que usan identidades
// sintéticas (fixedUUID) inexistentes en auth_db: un Auth real respondería
// siempre allowed=false. NO conoce ningún catálogo rol->permiso: responde
// allowed=true salvo que el propio test declare una negación puntual
// (userID -> permiso). Qué permisos tiene realmente cada rol se prueba contra
// el Auth Service real en TestHasPermissionIntegration (mismo paquete).
type scriptedAuthClient struct {
	denied map[string]map[string]bool
}

func newScriptedAuthClient(denied map[string]map[string]bool) authclient.Auth {
	return &scriptedAuthClient{denied: denied}
}

func (c *scriptedAuthClient) HasPermission(_ context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	return &authclient.HasPermissionResponse{Allowed: !c.denied[in.UserId][in.Permission]}, nil
}

func (c *scriptedAuthClient) Ping(context.Context, *authclient.Request, ...grpc.CallOption) (*authclient.Response, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) Register(context.Context, *authclient.RegisterRequest, ...grpc.CallOption) (*authclient.RegisterResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) Login(context.Context, *authclient.LoginRequest, ...grpc.CallOption) (*authclient.LoginResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) RefreshToken(context.Context, *authclient.RefreshTokenRequest, ...grpc.CallOption) (*authclient.RefreshTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) Logout(context.Context, *authclient.LogoutRequest, ...grpc.CallOption) (*authclient.LogoutResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) ValidateToken(context.Context, *authclient.ValidateTokenRequest, ...grpc.CallOption) (*authclient.ValidateTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

// ListRoles/ListPermissions/GetRolePermissions (Paso 21B): añadidos a
// authclient.Auth -- no usados por esta suite, solo necesarios para que
// este doble siga satisfaciendo la interfaz completa.
func (c *scriptedAuthClient) ListRoles(context.Context, *authclient.ListRolesRequest, ...grpc.CallOption) (*authclient.ListRolesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) ListPermissions(context.Context, *authclient.ListPermissionsRequest, ...grpc.CallOption) (*authclient.ListPermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) GetRolePermissions(context.Context, *authclient.GetRolePermissionsRequest, ...grpc.CallOption) (*authclient.GetRolePermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

// UpdateRolePermissions (Paso 21C): añadido a authclient.Auth -- no usado
// por esta suite, solo necesario para que este doble siga satisfaciendo la
// interfaz completa.
func (c *scriptedAuthClient) UpdateRolePermissions(context.Context, *authclient.UpdateRolePermissionsRequest, ...grpc.CallOption) (*authclient.UpdateRolePermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

// GetMyPermissions: añadido a authclient.Auth -- no usado por esta suite,
// solo necesario para que este doble siga satisfaciendo la interfaz
// completa.
func (c *scriptedAuthClient) GetMyPermissions(context.Context, *authclient.GetMyPermissionsRequest, ...grpc.CallOption) (*authclient.GetMyPermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) UpdateUserRole(context.Context, *authclient.UpdateUserRoleRequest, ...grpc.CallOption) (*authclient.UpdateUserRoleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) CreateUser(context.Context, *authclient.CreateUserRequest, ...grpc.CallOption) (*authclient.CreateUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}
