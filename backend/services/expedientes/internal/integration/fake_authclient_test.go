package integration

import (
	"context"
	"strings"

	"auth/authclient"
	"expedientes/internal/security"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// rolePermissionsReplica es una copia de la MISMA matriz rol->permiso ya
// sembrada en auth_db por la migración
// 005_seed_permissions_and_role_permissions.sql (Paso 9) — no se inventa
// ninguna regla nueva acá, solo se refleja la existente para poder probar
// esta suite sin depender de un Auth Service real corriendo.
var rolePermissionsReplica = map[string]map[string]bool{
	"ADMIN": set(
		"dashboard.view", "expedientes.view", "expedientes.create", "expedientes.update",
		"expedientes.change_estado", "expedientes.delete", "documentos.view", "documentos.create",
		"documentos.delete", "derivaciones.view", "derivaciones.create", "seguimiento.view",
		"reportes.view", "usuarios.view", "roles.view", "configuracion.view",
	),
	"DIRECTOR": set(
		"dashboard.view", "expedientes.view", "expedientes.update", "expedientes.change_estado",
		"expedientes.delete", "documentos.view", "documentos.create", "documentos.delete",
		"derivaciones.view", "derivaciones.create", "seguimiento.view", "reportes.view",
	),
	"SUBDIRECTOR": set(
		"dashboard.view", "expedientes.view", "expedientes.update", "expedientes.change_estado",
		"expedientes.delete", "documentos.view", "documentos.create", "documentos.delete",
		"derivaciones.view", "derivaciones.create", "seguimiento.view", "reportes.view",
	),
	"SECRETARIA": set(
		"dashboard.view", "expedientes.view", "expedientes.create", "expedientes.update",
		"expedientes.change_estado", "expedientes.delete", "documentos.view", "documentos.create",
		"documentos.delete", "derivaciones.view", "derivaciones.create", "seguimiento.view",
	),
	"DOCENTE": set(
		"dashboard.view", "expedientes.view", "expedientes.update", "expedientes.change_estado",
		"expedientes.delete", "documentos.view", "documentos.create", "documentos.delete",
		"derivaciones.view", "derivaciones.create", "seguimiento.view",
	),
	"AUXILIAR": set(
		"dashboard.view", "expedientes.view", "expedientes.update", "expedientes.change_estado",
		"expedientes.delete", "documentos.view", "documentos.create", "documentos.delete",
		"derivaciones.view", "derivaciones.create", "seguimiento.view",
	),
	"SOLICITANTE": set(
		"dashboard.view", "solicitudes.create", "expedientes.view_own", "documentos.view_own",
		"documentos.create", "seguimiento.view_own",
	),
}

func set(codes ...string) map[string]bool {
	m := make(map[string]bool, len(codes))
	for _, c := range codes {
		m[c] = true
	}
	return m
}

// replicaAuthClient es un doble de prueba SOLO para
// TestExpedientesIntegration: esa suite usa identidades sintéticas
// (fixedUUID) que nunca existen en auth_db, así que un AuthClient real
// respondería siempre allowed=false por "usuario inexistente" —
// impidiendo probar las reglas de negocio (área/ownership/estado/CAS/F2/
// F4) que esa suite en realidad verifica, y que son ortogonales al
// catálogo de permisos (ese catálogo ya tiene su propia prueba real
// contra Auth Service, ver TestHasPermissionIntegration en este mismo
// paquete).
//
// En vez de "permitir siempre" (lo que ocultaría casos reales, como que
// un SOLICITANTE no debe poder editar un expediente), este doble lee el
// MISMO JWT que ya autenticó la llamada (reenviado como metadata saliente
// por authorization.RequirePermission) y aplica rolePermissionsReplica —
// la misma matriz real, no una regla inventada.
type replicaAuthClient struct {
	validator *security.JWTValidator
}

func newReplicaAuthClient(validator *security.JWTValidator) authclient.Auth {
	return &replicaAuthClient{validator: validator}
}

func (c *replicaAuthClient) HasPermission(ctx context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	role, err := roleFromOutgoingToken(ctx, c.validator)
	if err != nil {
		return nil, err
	}
	return &authclient.HasPermissionResponse{Allowed: rolePermissionsReplica[role][in.Permission]}, nil
}

func roleFromOutgoingToken(ctx context.Context, validator *security.JWTValidator) (string, error) {
	values, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	authValues := values.Get("authorization")
	if len(authValues) == 0 {
		return "", status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	parts := strings.Fields(authValues[0])
	if len(parts) != 2 {
		return "", status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	claims, err := validator.Validate(parts[1])
	if err != nil {
		return "", status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	return claims.Role, nil
}

func (c *replicaAuthClient) Ping(context.Context, *authclient.Request, ...grpc.CallOption) (*authclient.Response, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *replicaAuthClient) Register(context.Context, *authclient.RegisterRequest, ...grpc.CallOption) (*authclient.RegisterResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *replicaAuthClient) Login(context.Context, *authclient.LoginRequest, ...grpc.CallOption) (*authclient.LoginResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *replicaAuthClient) RefreshToken(context.Context, *authclient.RefreshTokenRequest, ...grpc.CallOption) (*authclient.RefreshTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *replicaAuthClient) Logout(context.Context, *authclient.LogoutRequest, ...grpc.CallOption) (*authclient.LogoutResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *replicaAuthClient) ValidateToken(context.Context, *authclient.ValidateTokenRequest, ...grpc.CallOption) (*authclient.ValidateTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

// ListRoles/ListPermissions/GetRolePermissions (Paso 21B): añadidos a
// authclient.Auth -- no usados por esta suite, solo necesarios para que
// este doble siga satisfaciendo la interfaz completa.
func (c *replicaAuthClient) ListRoles(context.Context, *authclient.ListRolesRequest, ...grpc.CallOption) (*authclient.ListRolesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *replicaAuthClient) ListPermissions(context.Context, *authclient.ListPermissionsRequest, ...grpc.CallOption) (*authclient.ListPermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *replicaAuthClient) GetRolePermissions(context.Context, *authclient.GetRolePermissionsRequest, ...grpc.CallOption) (*authclient.GetRolePermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

// UpdateRolePermissions (Paso 21C): añadido a authclient.Auth -- no usado
// por esta suite, solo necesario para que este doble siga satisfaciendo la
// interfaz completa.
func (c *replicaAuthClient) UpdateRolePermissions(context.Context, *authclient.UpdateRolePermissionsRequest, ...grpc.CallOption) (*authclient.UpdateRolePermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}
