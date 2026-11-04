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
// un sistema: al menos uno, sin repetir y SOLO de lectura mínima
// (GrantableServicePermissions: estudiantes.basic.view y
// estudiantes.search.basic). Un sistema nuevo nunca recibe estudiantes.view,
// datos personales ni permisos de escritura.
func validarPermisosSistema(permisos []string) ([]string, error) {
	scope := security.NormalizeScope(permisos)
	if scope == "" {
		return nil, status.Error(codes.InvalidArgument, "debe asignar al menos un permiso")
	}
	lista := strings.Fields(scope)
	for _, p := range lista {
		if !security.GrantableServicePermissions[p] {
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

// permisoSistemaStore: lo mínimo del repositorio para otorgar y retirar
// permisos (se reemplaza por un doble en las pruebas).
type permisoSistemaStore interface {
	FindByClientID(ctx context.Context, clientID string) (*repository.ServiceClient, error)
	AddPermission(ctx context.Context, serviceClientID, codigo string) (bool, error)
	RemovePermission(ctx context.Context, serviceClientID, codigo string) (bool, error)
}

// cambiarPermisoSistema otorga (otorgar=true) o retira un permiso de un
// sistema consumidor existente, sin recrearlo ni tocar sus credenciales.
// Solo permisos de security.GrantableServicePermissions, también al retirar.
// Idempotente:
// changed=false si no había nada que hacer. Cada resultado queda en el log
// de auditoría (sin secretos: solo administrador, client_id y permiso).
func cambiarPermisoSistema(ctx context.Context, store permisoSistemaStore, adminID, clientID, codigo string, otorgar bool) (*auth.ServiceClientPermissionResponse, error) {
	operacion := "retirar"
	if otorgar {
		operacion = "otorgar"
	}
	clientID, codigo = strings.TrimSpace(clientID), strings.TrimSpace(codigo)
	auditar := func(resultado string, cambio bool) {
		idLog := "<formato inválido>"
		if security.ValidClientIDFormat(clientID) {
			idLog = clientID
		}
		logx.WithContext(ctx).Infow("auditoría: permiso de sistema consumidor",
			logx.Field("operacion", operacion),
			logx.Field("resultado", resultado),
			logx.Field("cambio", cambio),
			logx.Field("admin_id", adminID),
			logx.Field("client_id", idLog),
			logx.Field("permiso", codigoParaLog(codigo)),
		)
	}

	if !security.ValidClientIDFormat(clientID) {
		auditar("rechazado: client_id inválido", false)
		return nil, status.Error(codes.InvalidArgument, "client_id inválido")
	}
	if !security.GrantableServicePermissions[codigo] {
		auditar("rechazado: permiso no asignable", false)
		return nil, status.Errorf(codes.InvalidArgument, "el permiso %q no se puede asignar a un sistema", codigoParaLog(codigo))
	}
	client, err := store.FindByClientID(ctx, clientID)
	if errors.Is(err, repository.ErrServiceClientNotFound) {
		auditar("rechazado: sistema inexistente", false)
		return nil, status.Error(codes.NotFound, "sistema consumidor no encontrado")
	}
	if err != nil {
		logx.WithContext(ctx).Errorf("%s permiso de sistema: no se pudo consultar el cliente: %v", operacion, err)
		return nil, status.Error(codes.Internal, "no se pudo consultar el sistema consumidor")
	}
	if otorgar && client.Estado != repository.ServiceClientActivo {
		auditar("rechazado: sistema revocado", false)
		return nil, status.Error(codes.FailedPrecondition, "el sistema consumidor está revocado: no se le otorgan permisos")
	}

	var cambio bool
	if otorgar {
		cambio, err = store.AddPermission(ctx, client.ID, codigo)
	} else {
		cambio, err = store.RemovePermission(ctx, client.ID, codigo)
	}
	if errors.Is(err, repository.ErrPermisoNoCatalogado) {
		auditar("rechazado: permiso fuera del catálogo", false)
		return nil, status.Error(codes.FailedPrecondition, "el permiso no está registrado o está inactivo en el catálogo")
	}
	if err != nil {
		logx.WithContext(ctx).Errorf("%s permiso de sistema: %v", operacion, err)
		return nil, status.Error(codes.Internal, "no se pudo actualizar los permisos del sistema consumidor")
	}

	actualizado, err := store.FindByClientID(ctx, clientID)
	if err != nil {
		logx.WithContext(ctx).Errorf("%s permiso de sistema: no se pudo releer el cliente: %v", operacion, err)
		return nil, status.Error(codes.Internal, "no se pudo consultar el sistema consumidor")
	}
	if cambio {
		auditar("aplicado", true)
	} else {
		auditar("sin cambios (idempotente)", false)
	}
	return &auth.ServiceClientPermissionResponse{Client: serviceClientProto(actualizado), Changed: cambio}, nil
}

// codigoParaLog: los códigos de permiso son identificadores cortos; un valor
// con otra forma (posible dato pegado por error) no se registra.
func codigoParaLog(codigo string) string {
	if len(codigo) > 0 && len(codigo) <= 100 && strings.Trim(codigo, "abcdefghijklmnopqrstuvwxyz._") == "" {
		return codigo
	}
	return "<formato inválido>"
}
