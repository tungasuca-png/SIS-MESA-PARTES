package integration

import (
	"context"
	"strings"

	"auth/authclient"
	"expedientes/expedientesclient"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// scriptedAuthClient es un doble de Auth SOLO para las suites de REGLAS DE
// NEGOCIO de documentos (área/ownership/estado/etc.), que usan identidades
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

func (c *scriptedAuthClient) IssueServiceToken(context.Context, *authclient.IssueServiceTokenRequest, ...grpc.CallOption) (*authclient.IssueServiceTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) CreateServiceClient(context.Context, *authclient.CreateServiceClientRequest, ...grpc.CallOption) (*authclient.CreateServiceClientResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) RevokeServiceClient(context.Context, *authclient.RevokeServiceClientRequest, ...grpc.CallOption) (*authclient.RevokeServiceClientResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) ListServiceClients(context.Context, *authclient.ListServiceClientsRequest, ...grpc.CallOption) (*authclient.ListServiceClientsResponse, error) {
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
	// rolesFueraDeArea simula a Expedientes Service cuando el expediente no
	// está en el área del rol que llama (DOCENTE, AUXILIAR, ...): Get responde
	// PermissionDenied y List no lo incluye. El rol se lee del JWT reenviado.
	rolesFueraDeArea map[string]bool
}

func (c *fakeExpedientesClient) fueraDeArea(ctx context.Context) bool {
	md, _ := metadata.FromOutgoingContext(ctx)
	for _, v := range md.Get("authorization") {
		claims := jwt.MapClaims{}
		if _, _, err := jwt.NewParser().ParseUnverified(strings.TrimPrefix(v, "Bearer "), claims); err == nil {
			if role, _ := claims["role"].(string); c.rolesFueraDeArea[role] {
				return true
			}
		}
	}
	return false
}

func (c *fakeExpedientesClient) GetExpediente(ctx context.Context, in *expedientesclient.GetExpedienteRequest, _ ...grpc.CallOption) (*expedientesclient.GetExpedienteResponse, error) {
	if c.fueraDeArea(ctx) {
		return nil, status.Error(codes.PermissionDenied, "no tiene acceso a este expediente")
	}
	if in.Id != c.expedienteID {
		return nil, status.Error(codes.NotFound, "expediente no encontrado")
	}
	return &expedientesclient.GetExpedienteResponse{
		Expediente: &expedientesclient.Expediente{Id: c.expedienteID, SolicitanteId: c.solicitanteID, Estado: "PENDIENTE"},
	}, nil
}

func (c *fakeExpedientesClient) ListExpedientes(ctx context.Context, _ *expedientesclient.ListExpedientesRequest, _ ...grpc.CallOption) (*expedientesclient.ListExpedientesResponse, error) {
	if c.fueraDeArea(ctx) {
		return &expedientesclient.ListExpedientesResponse{}, nil
	}
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
