package integration

import (
	"context"
	"strings"

	"auth/authclient"
	"documentos/internal/security"
	"expedientes/expedientesclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// rolePermissionsReplica es una copia de la MISMA matriz rol->permiso ya
// sembrada en auth_db por la migración
// 005_seed_permissions_and_role_permissions.sql (Paso 9) — no se inventa
// ninguna regla nueva acá, solo se refleja la existente para poder probar
// esta suite (identidades sintéticas que no existen en auth_db) sin
// depender de un Auth Service real corriendo. Mismo patrón ya usado en
// Expedientes (Paso 13B).
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
// TestDocumentosIntegration: esa suite usa identidades sintéticas
// (fixedUUID) que nunca existen en auth_db, así que un AuthClient real
// respondería siempre allowed=false por "usuario inexistente". En vez de
// "permitir siempre" (lo que ocultaría casos reales, como que un
// SOLICITANTE no debe poder subir PROVEIDO ni eliminar), este doble lee
// el MISMO JWT que ya autenticó la llamada (reenviado como metadata
// saliente por authorization.HasPermission/RequirePermission) y aplica
// rolePermissionsReplica — la misma matriz real, no una regla inventada.
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

// fakeExpedientesClient (Paso 19C) es un doble de prueba SOLO para
// TestDocumentosIntegration: esa suite usa un expediente_id y un
// solicitante_id SINTÉTICOS (fixedUUID) que nunca existen en
// expedientes_db, así que un ExpedientesClient real respondería siempre
// NotFound. Este doble fija, de forma local a esta suite, la MISMA
// relación expediente/solicitante que el propio test ya construye
// (expedienteID pertenece a solicitanteID) — no inventa ninguna regla
// nueva, solo evita depender de un Expedientes Service real corriendo. El
// ownership real (Expedientes.GetExpediente/ListExpedientes) se prueba de
// verdad, contra un Expedientes Service real, en
// TestHasPermissionIntegration (Pasos 19B/19C).
// estado (Paso 19E): DeleteDocumento ahora también exige que el expediente
// siga PENDIENTE. Esta suite (TestDocumentosIntegration) no ejerce ningún
// caso de estado distinto -- solo necesita que el ADMIN de esa prueba
// pueda eliminar el PROVEIDO como ya hacía antes de este paso, así que el
// valor fijo es "PENDIENTE" (el mismo caso real de estado que se prueba a
// fondo, con Expedientes real, en TestHasPermissionIntegration).
type fakeExpedientesClient struct {
	expedienteID  string
	solicitanteID string
}

func (c *fakeExpedientesClient) GetExpediente(_ context.Context, in *expedientesclient.GetExpedienteRequest, _ ...grpc.CallOption) (*expedientesclient.GetExpedienteResponse, error) {
	if in.Id != c.expedienteID {
		return nil, status.Error(codes.NotFound, "expediente no encontrado")
	}
	return &expedientesclient.GetExpedienteResponse{
		Expediente: &expedientesclient.Expediente{Id: c.expedienteID, SolicitanteId: c.solicitanteID, Estado: "PENDIENTE"},
	}, nil
}

func (c *fakeExpedientesClient) ListExpedientes(context.Context, *expedientesclient.ListExpedientesRequest, ...grpc.CallOption) (*expedientesclient.ListExpedientesResponse, error) {
	return &expedientesclient.ListExpedientesResponse{
		Expedientes: []*expedientesclient.Expediente{{Id: c.expedienteID, SolicitanteId: c.solicitanteID}},
		Total:       1,
	}, nil
}

func (c *fakeExpedientesClient) CreateExpediente(context.Context, *expedientesclient.CreateExpedienteRequest, ...grpc.CallOption) (*expedientesclient.CreateExpedienteResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *fakeExpedientesClient) UpdateExpediente(context.Context, *expedientesclient.UpdateExpedienteRequest, ...grpc.CallOption) (*expedientesclient.UpdateExpedienteResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *fakeExpedientesClient) ChangeEstado(context.Context, *expedientesclient.ChangeEstadoRequest, ...grpc.CallOption) (*expedientesclient.ChangeEstadoResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *fakeExpedientesClient) DeleteExpediente(context.Context, *expedientesclient.DeleteExpedienteRequest, ...grpc.CallOption) (*expedientesclient.DeleteExpedienteResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *fakeExpedientesClient) UpdateArea(context.Context, *expedientesclient.UpdateAreaRequest, ...grpc.CallOption) (*expedientesclient.UpdateAreaResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *fakeExpedientesClient) DerivarExpediente(context.Context, *expedientesclient.DerivarExpedienteRequest, ...grpc.CallOption) (*expedientesclient.DerivarExpedienteResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *fakeExpedientesClient) RechazarExpediente(context.Context, *expedientesclient.RechazarExpedienteRequest, ...grpc.CallOption) (*expedientesclient.RechazarExpedienteResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *fakeExpedientesClient) CorregirExpediente(context.Context, *expedientesclient.CorregirExpedienteRequest, ...grpc.CallOption) (*expedientesclient.CorregirExpedienteResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *fakeExpedientesClient) ResolverExpediente(context.Context, *expedientesclient.ResolverExpedienteRequest, ...grpc.CallOption) (*expedientesclient.ResolverExpedienteResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

// ValidateDerivacion (Paso 20B.2): añadido a expedientesclient.Expedientes
// -- no usado por Documentos, solo necesario para que este doble siga
// satisfaciendo la interfaz completa.
func (c *fakeExpedientesClient) ValidateDerivacion(context.Context, *expedientesclient.ValidateDerivacionRequest, ...grpc.CallOption) (*expedientesclient.ValidateDerivacionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}
