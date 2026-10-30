package logic

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"auth/auth"
	"auth/internal/repository"
	"auth/internal/security"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Client credentials para sistemas consumidores. Ningún error distingue entre
// client_id inexistente, secreto incorrecto o cliente revocado (no se revela
// qué clientes existen); solo un cliente auténtico y activo sin permisos
// recibe un mensaje propio.

const mensajeCredencialesCliente = "credenciales de cliente inválidas"

// serviceClientStore: lo mínimo del repositorio que necesita la emisión (se
// reemplaza por un doble en las pruebas).
type serviceClientStore interface {
	FindByClientID(ctx context.Context, clientID string) (*repository.ServiceClient, error)
	TouchLastUsed(ctx context.Context, id string) error
}

// hashDeRelleno: se compara contra él cuando el cliente no existe, para que
// la respuesta tarde lo mismo que con un secreto incorrecto.
var hashDeRelleno = security.HashClientSecret("cs_relleno")

func emitirTokenServicio(ctx context.Context, store serviceClientStore, issuer *security.ServiceTokenIssuer, clientID, clientSecret string) (*auth.IssueServiceTokenResponse, error) {
	logger := logx.WithContext(ctx)
	if issuer == nil {
		return nil, status.Error(codes.Unavailable, "la autenticación de sistemas no está habilitada")
	}
	clientID = strings.TrimSpace(clientID)
	if clientID == "" || clientSecret == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id y client_secret son obligatorios")
	}
	idLog := "<formato inválido>"
	if security.ValidClientIDFormat(clientID) {
		idLog = clientID
	}

	client, err := store.FindByClientID(ctx, clientID)
	if err != nil && !errors.Is(err, repository.ErrServiceClientNotFound) {
		logger.Errorf("token de servicio: no se pudo consultar el cliente %s: %v", idLog, err)
		return nil, status.Error(codes.Internal, "no se pudo validar el cliente")
	}
	esperado := hashDeRelleno
	if client != nil {
		esperado = client.SecretHash
	}
	coincide := subtle.ConstantTimeCompare(security.HashClientSecret(clientSecret), esperado) == 1
	if client == nil || !coincide || client.Estado != repository.ServiceClientActivo {
		logger.Infof("token de servicio rechazado: client_id=%s", idLog)
		return nil, status.Error(codes.Unauthenticated, mensajeCredencialesCliente)
	}

	// Defensa en profundidad: aunque la base tuviera otro permiso asociado,
	// el token solo lleva permisos asignables a sistemas.
	permisos := make([]string, 0, len(client.Permisos))
	for _, p := range client.Permisos {
		if security.AssignableServicePermissions[p] {
			permisos = append(permisos, p)
		}
	}
	if len(permisos) == 0 {
		logger.Infof("token de servicio rechazado (sin permisos): client_id=%s", idLog)
		return nil, status.Error(codes.PermissionDenied, "el cliente no tiene permisos asignados")
	}

	token, scope, duracion, err := issuer.Issue(client.ID, client.ClientID, permisos)
	if err != nil {
		logger.Errorf("token de servicio: no se pudo firmar para %s: %v", idLog, err)
		return nil, status.Error(codes.Internal, "no se pudo emitir el token")
	}
	if err := store.TouchLastUsed(ctx, client.ID); err != nil {
		// No bloquea la emisión: last_used_at es informativo.
		logger.Errorf("token de servicio: no se pudo registrar el uso de %s: %v", idLog, err)
	}
	logger.Infof("token de servicio emitido: client_id=%s scope=%q", idLog, scope)
	return &auth.IssueServiceTokenResponse{
		AccessToken: token,
		ExpiresIn:   int64(duracion / time.Second),
		Scope:       scope,
	}, nil
}

// validarPermisosSistema normaliza y valida los permisos pedidos al registrar
// un sistema: al menos uno, todos asignables a sistemas, sin repetir.
func validarPermisosSistema(permisos []string) ([]string, error) {
	scope := security.NormalizeScope(permisos)
	if scope == "" {
		return nil, status.Error(codes.InvalidArgument, "debe asignar al menos un permiso")
	}
	lista := strings.Fields(scope)
	for _, p := range lista {
		if !security.AssignableServicePermissions[p] {
			return nil, status.Errorf(codes.InvalidArgument, "el permiso %q no se puede asignar a un sistema", p)
		}
	}
	return lista, nil
}

func serviceClientProto(c *repository.ServiceClient) *auth.ServiceClient {
	out := &auth.ServiceClient{
		Id:        c.ID,
		Nombre:    c.Nombre,
		ClientId:  c.ClientID,
		Estado:    c.Estado,
		Permisos:  c.Permisos,
		CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339),
	}
	if c.LastUsedAt != nil {
		out.LastUsedAt = c.LastUsedAt.UTC().Format(time.RFC3339)
	}
	if c.RevokedAt != nil {
		out.RevokedAt = c.RevokedAt.UTC().Format(time.RFC3339)
	}
	return out
}
