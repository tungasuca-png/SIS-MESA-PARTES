package authorization

import (
	"context"

	"auth/authclient"
	"expedientes/internal/interceptor"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// PermissionClient es la porción mínima de authclient.Auth que este paquete
// necesita — evita acoplar authorization.go (y sus reglas de negocio, que
// NO cambian en este paso) a la interfaz completa del cliente de Auth.
type PermissionClient interface {
	HasPermission(ctx context.Context, in *authclient.HasPermissionRequest, opts ...grpc.CallOption) (*authclient.HasPermissionResponse, error)
}

// RequirePermission consulta Auth.HasPermission (Paso 12) y es FAIL-CLOSED:
// cualquier error de comunicación con Auth Service se propaga tal cual —
// nunca se traduce en "permitido" ni cae de vuelta al rol local — y
// allowed=false se traduce en PermissionDenied. Esto reemplaza ÚNICAMENTE
// el gate de permiso basado en rol (CanCreate/CanUpdate/CanChangeEstado/
// CanDelete/CanUpdateArea); las reglas de negocio de este paquete
// (IsInternal, CanViewAll, AreaDelRol) y las de internal/estados
// (CanTransition, CanDerivar, EsF2, EsF4) siguen aplicándose exactamente
// igual en cada Logic, después de esta llamada.
func RequirePermission(ctx context.Context, client PermissionClient, userID, permission string) error {
	outCtx := ctx
	if token, ok := interceptor.TokenFromContext(ctx); ok {
		// Auth Service exige autenticación en HasPermission (igual que en
		// Logout) — se reenvía el MISMO JWT que ya autenticó esta llamada
		// a Expedientes, nunca uno nuevo.
		outCtx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	resp, err := client.HasPermission(outCtx, &authclient.HasPermissionRequest{
		UserId:     userID,
		Permission: permission,
	})
	if err != nil {
		return err
	}
	if resp == nil || !resp.Allowed {
		return status.Error(codes.PermissionDenied, "el usuario no tiene el permiso requerido")
	}
	return nil
}
