package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"auth/internal/repository"
	"auth/internal/security"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Acceso con Google, solo SOLICITANTES. Tres flujos separados:
//   - LOGIN: identidad (sub) ya vinculada a una cuenta SOLICITANTE activa.
//   - REGISTRO: identidad nueva -> formulario obligatorio con DNI.
//   - VINCULACION: cuenta SOLICITANTE existente, autenticada y reautenticada
//     con su clave, que agrega Google.
// La identidad es SIEMPRE el "sub" del ID token; el correo nunca vincula.

const (
	duracionCodigoLogin       = 2 * time.Minute
	duracionCodigoRegistro    = 30 * time.Minute
	duracionCodigoVinculacion = 10 * time.Minute
)

// Mensajes públicos: no revelan rol, estado ni datos de otras cuentas.
const (
	MensajeDNIRegistrado = "Este DNI ya tiene una cuenta registrada. Inicia sesión con tus credenciales institucionales. " +
		"Si deseas utilizar Google, podrás vincularlo desde tu perfil después de iniciar sesión."
	MensajeCorreoExistente = "Ya existe una cuenta con este correo. Inicia sesión con tus credenciales institucionales " +
		"y vincula Google desde tu perfil."
	MensajeCuentaNoDisponible = "Esta cuenta no puede iniciar sesión con Google. Comunícate con la institución."
	MensajeCodigoInvalido     = "El enlace de acceso venció o ya fue utilizado. Vuelve a intentarlo."
)

// errGoogle: "google_<clave>: <mensaje>". La clave es estable (el Gateway y
// el frontend la usan); el mensaje es para la persona.
func errGoogle(c codes.Code, clave, mensaje string) error {
	return status.Error(c, "google_"+clave+": "+mensaje)
}

func errCodigoInvalido() error {
	return errGoogle(codes.Unauthenticated, "codigo_invalido", MensajeCodigoInvalido)
}

func errNoConfigurado() error {
	return errGoogle(codes.Unavailable, "no_configurado", "El acceso con Google no está habilitado en este momento.")
}

// errProveedor traduce los errores del canje y la validación del ID token.
func errProveedor(err error) error {
	switch {
	case errors.Is(err, security.ErrGoogleNoConfigurado):
		return errNoConfigurado()
	case errors.Is(err, security.ErrGoogleNoDisponible):
		return errGoogle(codes.Unavailable, "no_disponible", "Google no respondió. Inténtalo nuevamente en unos minutos.")
	case errors.Is(err, security.ErrGoogleCodigo):
		return errGoogle(codes.Unauthenticated, "codigo_rechazado", "Google rechazó la autorización. Vuelve a intentarlo.")
	case errors.Is(err, security.ErrGoogleToken), errors.Is(err, security.ErrGoogleNonce):
		return errGoogle(codes.Unauthenticated, "token_invalido", "No se pudo verificar la identidad de Google. Vuelve a intentarlo.")
	}
	return status.Error(codes.Internal, "no se pudo procesar el acceso con Google")
}

// cuentaSolicitanteDisponible aplica las mismas reglas que el login: cuenta
// activa, sin bloqueo vigente y, además, rol SOLICITANTE.
func cuentaSolicitanteDisponible(ctx context.Context, svcCtx *svc.ServiceContext, usuarioID string) (*repository.User, string, error) {
	user, err := svcCtx.UserRepository.FindByID(ctx, usuarioID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, "", errGoogle(codes.PermissionDenied, "cuenta_no_disponible", MensajeCuentaNoDisponible)
		}
		return nil, "", status.Error(codes.Internal, "no se pudo procesar el acceso con Google")
	}
	if !user.Estado {
		return nil, "", errGoogle(codes.PermissionDenied, "cuenta_no_disponible", MensajeCuentaNoDisponible)
	}
	if user.LockedUntil.Valid && time.Now().Before(user.LockedUntil.Time) {
		return nil, "", accountLockedError(user.LockedUntil.Time)
	}
	roles, err := svcCtx.UserRepository.GetRoles(ctx, user.ID)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "no se pudo obtener el rol del usuario")
	}
	if len(roles) != 1 || roles[0] != repository.RolSolicitante {
		return nil, "", errGoogle(codes.PermissionDenied, "cuenta_no_disponible", MensajeCuentaNoDisponible)
	}
	return user, roles[0], nil
}

// nuevoCodigo crea un código de un solo uso y devuelve su valor (solo el
// hash queda en la base).
func nuevoCodigo(ctx context.Context, svcCtx *svc.ServiceContext, c repository.NuevoCodigo, duracion time.Duration) (string, error) {
	valor, hash, err := security.GenerateRefreshToken()
	if err != nil {
		return "", status.Error(codes.Internal, "no se pudo generar el código")
	}
	c.Hash = hash
	c.ExpiresAt = time.Now().Add(duracion)
	if err := svcCtx.GoogleRepository.CreateCode(ctx, c); err != nil {
		return "", status.Error(codes.Internal, "no se pudo guardar el código")
	}
	return valor, nil
}

// hashCodigo: el valor recibido se compara siempre por su hash.
func hashCodigo(valor string) ([]byte, bool) {
	valor = strings.TrimSpace(valor)
	if valor == "" || len(valor) > 200 {
		return nil, false
	}
	return security.HashRefreshToken(valor), true
}

// evento: auditoría de mejor esfuerzo (sin datos sensibles).
func evento(ctx context.Context, svcCtx *svc.ServiceContext, usuarioID, nombre, resultado, motivo string) {
	if svcCtx.GoogleRepository == nil {
		return
	}
	if err := svcCtx.GoogleRepository.LogEvent(ctx, usuarioID, nombre, resultado, motivo); err != nil {
		logx.WithContext(ctx).Errorf("auditoría de autenticación (%s): no se pudo registrar el evento", nombre)
	}
}

// motivoDe: clave estable de un error google_<clave> (para la auditoría).
func motivoDe(err error) string {
	msg := status.Convert(err).Message()
	if i := strings.Index(msg, ":"); strings.HasPrefix(msg, "google_") && i > 0 {
		return msg[len("google_"):i]
	}
	return status.Code(err).String()
}

// credencialesInternas: usuario aleatorio y clave aleatoria descartada para
// las cuentas creadas con Google (usuarios.password_hash es obligatorio).
// Nadie conoce esa clave y, sin DNI conocido por un tercero, no sirve para
// el login tradicional; la institución puede asignar una clave después.
func credencialesInternas() (username, passwordHash string, err error) {
	b := make([]byte, 8)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	secreto, _, err := security.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secreto), bcrypt.DefaultCost)
	if err != nil {
		return "", "", err
	}
	return "sol_" + hex.EncodeToString(b), string(hash), nil
}
