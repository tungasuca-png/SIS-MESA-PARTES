package authorization

import (
	"context"

	"auth/authclient"
	"documentos/internal/interceptor"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// PermissionClient es la porción mínima de authclient.Auth que este
// paquete necesita — evita acoplar authorization.go (y sus reglas de
// negocio, que NO cambian en este paso) a la interfaz completa del
// cliente de Auth. Mismo patrón ya usado en Expedientes (Paso 13B).
type PermissionClient interface {
	HasPermission(ctx context.Context, in *authclient.HasPermissionRequest, opts ...grpc.CallOption) (*authclient.HasPermissionResponse, error)
}

// HasPermission consulta Auth.HasPermission y devuelve el booleano crudo.
// Es FAIL-CLOSED: cualquier error de comunicación con Auth Service se
// propaga tal cual — nunca se traduce en "permitido" ni cae de vuelta al
// rol local.
func HasPermission(ctx context.Context, client PermissionClient, userID, permission string) (bool, error) {
	outCtx := ctx
	if token, ok := interceptor.TokenFromContext(ctx); ok {
		// Auth Service exige autenticación en HasPermission (igual que en
		// Logout) — se reenvía el MISMO JWT que ya autenticó esta llamada
		// a Documentos, nunca uno nuevo.
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
// PermissionDenied; cualquier error de Auth se propaga tal cual. No
// reemplaza ninguna regla de negocio existente (CanUpload/CanDelete/
// CanViewAll siguen aplicándose en cada Logic, después de esta llamada).
func RequirePermission(ctx context.Context, client PermissionClient, userID, permission string) error {
	allowed, err := HasPermission(ctx, client, userID, permission)
	if err != nil {
		return err
	}
	if !allowed {
		return status.Error(codes.PermissionDenied, "el usuario no tiene el permiso requerido")
	}
	return nil
}

// RequireAnyPermission exige AL MENOS UNO de los permisos indicados (p. ej.
// "documentos.view" o "documentos.view_own"). Fail-closed: si alguna
// consulta a Auth falla por comunicación, se detiene y propaga ese error
// de inmediato — NO sigue probando los permisos restantes como si el
// fallo fuera equivalente a "no lo tiene". Solo si TODOS los permisos
// consultados resultan expresamente denegados, devuelve PermissionDenied.
func RequireAnyPermission(ctx context.Context, client PermissionClient, userID string, permissions ...string) error {
	for _, permission := range permissions {
		allowed, err := HasPermission(ctx, client, userID, permission)
		if err != nil {
			return err
		}
		if allowed {
			return nil
		}
	}
	return status.Error(codes.PermissionDenied, "el usuario no tiene ninguno de los permisos requeridos")
}
