package authorization

import (
	"context"

	"auth/authclient"
	"estudiantes/internal/interceptor"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// PermissionClient es la porción mínima de authclient.Auth que este
// paquete necesita — evita acoplar authorization.go (y sus reglas de
// negocio, que NO cambian en este paso) a la interfaz completa del
// cliente de Auth. Mismo patrón ya usado en Expedientes/Documentos/
// Derivaciones.
type PermissionClient interface {
	HasPermission(ctx context.Context, in *authclient.HasPermissionRequest, opts ...grpc.CallOption) (*authclient.HasPermissionResponse, error)
}

// HasPermission consulta Auth.HasPermission y devuelve el booleano crudo.
// Es FAIL-CLOSED: cualquier error de comunicación con Auth Service se
// propaga tal cual — nunca se traduce en "permitido" ni cae de vuelta al
// rol local.
func HasPermission(ctx context.Context, client PermissionClient, userID, permission string) (bool, error) {
	// Sistema consumidor (client credentials): sus permisos son el scope del
	// token, firmado por Auth y de corta duración. No se consulta Auth (no es
	// un usuario) ni se reenvía su token.
	if service, ok := interceptor.ServiceFromContext(ctx); ok {
		return servicePermite(service, permission), nil
	}
	outCtx := ctx
	if token, ok := interceptor.TokenFromContext(ctx); ok {
		// Auth Service exige autenticación en HasPermission (igual que en
		// Logout) — se reenvía el MISMO JWT que ya autenticó esta llamada
		// a Estudiantes, nunca uno nuevo.
		outCtx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	resp, err := client.HasPermission(outCtx, &authclient.HasPermissionRequest{
		UserId:     userID,
		Permission: permission,
	})
	if err != nil {
		return false, err
	}
	return resp != nil && resp.Allowed, nil
}

// RequirePermission exige un único permiso: allowed=false se traduce en
// PermissionDenied; cualquier error de Auth se propaga tal cual.
// Es la autoridad de listar/buscar (estudiantes.view) y administrar perfiles
// (estudiantes.change_role); CanViewFullProfile es una regla de negocio aparte.
func RequirePermission(ctx context.Context, client PermissionClient, userID, permission string) error {
	allowed, err := HasPermission(ctx, client, userID, permission)
	if err != nil {
		return err
	}
	if !allowed {
		return status.Error(codes.PermissionDenied, "permission_denied: el usuario no tiene el permiso requerido")
	}
	return nil
}

// basicMethods: RPC que solo devuelven datos básicos (EstudianteBasico). En
// ellos, un sistema con estudiantes.basic.view cumple el requisito de
// estudiantes.view; en cualquier otro RPC, no.
var basicMethods = map[string]bool{
	"/estudiantes.Estudiantes/GetEstudiantesBasic": true,
	"/estudiantes.Estudiantes/ResolveEstudiante":   true,
}

func servicePermite(service *interceptor.ServiceIdentity, permission string) bool {
	if service.Permisos[permission] {
		return true
	}
	return permission == "estudiantes.view" && basicMethods[service.Method] && service.Permisos["estudiantes.basic.view"]
}
