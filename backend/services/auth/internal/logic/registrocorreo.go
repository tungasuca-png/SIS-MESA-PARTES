package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"auth/internal/config"
	"auth/internal/repository"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Registro manual de SOLICITANTES con verificación del correo:
//   1. EmailRegister: valida el formulario, guarda un registro pendiente y
//      envía un código de 6 dígitos (o, si el correo ya tiene cuenta, un
//      aviso; la respuesta es la misma para no revelar cuentas).
//   2. EmailVerify: con el código correcto crea la cuenta (correo
//      verificado, rol SOLICITANTE fijo) y entrega al Gateway un token para
//      crear el perfil en Usuarios Service.
//   3. EmailCompleteRegistration (perfil creado) o EmailCancelRegistration
//      (perfil rechazado: compensación).
// Mientras el registro no esté completado, la cuenta no puede iniciar sesión.

// Valores por defecto de config.RegistroCorreoConfig.
const (
	defDuracionCodigo = 10 * time.Minute
	defMaxIntentos    = 5
	defEsperaReenvio  = 60 * time.Second
	defMaxEnvios      = 5
	defMaxPorHora     = 5
)

type limitesRegistro struct {
	duracion time.Duration
	intentos int
	espera   time.Duration
	envios   int
	porHora  int
}

func limitesDe(c config.RegistroCorreoConfig) limitesRegistro {
	l := limitesRegistro{c.DuracionCodigo, c.MaxIntentos, c.EsperaReenvio, c.MaxEnvios, c.MaxRegistrosPorHora}
	if l.duracion <= 0 {
		l.duracion = defDuracionCodigo
	}
	if l.intentos <= 0 {
		l.intentos = defMaxIntentos
	}
	if l.espera <= 0 {
		l.espera = defEsperaReenvio
	}
	if l.envios <= 0 {
		l.envios = defMaxEnvios
	}
	if l.porHora <= 0 {
		l.porHora = defMaxPorHora
	}
	return l
}

var (
	reIDRegistro = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	reCodigo     = regexp.MustCompile(`^[0-9]{6}$`)
)

// errRegistro: "registro_<clave>: <mensaje>". La clave es estable (el
// Gateway y el frontend la usan); el mensaje es para la persona.
func errRegistro(c codes.Code, clave, mensaje string) error {
	return status.Error(c, "registro_"+clave+": "+mensaje)
}

const mensajeRegistroInvalido = "Este registro ya no está vigente. Vuelve a registrarte para recibir un código nuevo."

// registroDisponible: proveedor de correo y dependencias configurados.
func registroDisponible(svcCtx *svc.ServiceContext) error {
	if svcCtx.RegistroCorreoRepository == nil || svcCtx.CodigosVerificacion == nil ||
		svcCtx.Correo == nil || !svcCtx.Correo.Configurado() {
		return errRegistro(codes.Unavailable, "no_disponible", "El registro con correo no está habilitado en este momento.")
	}
	return nil
}

func idRegistroValido(id string) (string, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !reIDRegistro.MatchString(id) {
		return "", errRegistro(codes.InvalidArgument, "invalido", mensajeRegistroInvalido)
	}
	return id, nil
}

func codigoValido(codigo string) (string, error) {
	codigo = strings.TrimSpace(codigo)
	if !reCodigo.MatchString(codigo) {
		return "", errRegistro(codes.InvalidArgument, "codigo_formato", "El código tiene 6 dígitos numéricos.")
	}
	return codigo, nil
}

// normalizarCorreo: minúsculas, una sola dirección simple, máximo 254.
func normalizarCorreo(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	a, err := mail.ParseAddress(v)
	if v == "" || len(v) > 254 || err != nil || a.Address != v || strings.ContainsAny(v, "\r\n") {
		return "", errRegistro(codes.InvalidArgument, "datos_invalidos", "Ingresa un correo electrónico válido.")
	}
	return v, nil
}

// espacios: recorta y colapsa los espacios internos.
func espacios(v string) string { return strings.Join(strings.Fields(v), " ") }

// errDelRegistro traduce los errores del repositorio.
func errDelRegistro(err error) error {
	var incorrecto *repository.ErrCodigoIncorrecto
	var espera *repository.ErrEsperaReenvio
	switch {
	case errors.As(err, &incorrecto):
		quedan := "Te queda 1 intento."
		if incorrecto.Restantes != 1 {
			quedan = fmt.Sprintf("Te quedan %d intentos.", incorrecto.Restantes)
		}
		return errRegistro(codes.InvalidArgument, "codigo_incorrecto", "El código no es correcto. "+quedan)
	case errors.As(err, &espera):
		segundos := int(math.Ceil(espera.Restante.Seconds()))
		return errRegistro(codes.ResourceExhausted, "espera_reenvio",
			fmt.Sprintf("Podrás pedir otro código en %d segundos.", max(segundos, 1)))
	case repository.EsquemaPendiente(err):
		// Migraciones 024/025 pendientes: el registro manual no está disponible.
		logx.Errorf("registro con correo: migración pendiente (024/025)")
		return errRegistro(codes.Unavailable, "no_disponible", "El registro con correo no está habilitado en este momento.")
	case errors.Is(err, repository.ErrRegistroNoDisponible):
		return errRegistro(codes.InvalidArgument, "invalido", mensajeRegistroInvalido)
	case errors.Is(err, repository.ErrRegistroExpirado):
		return errRegistro(codes.InvalidArgument, "codigo_expirado", "El código venció. Solicita uno nuevo.")
	case errors.Is(err, repository.ErrRegistroIntentos):
		return errRegistro(codes.ResourceExhausted, "intentos_agotados",
			"Superaste el número de intentos para este código. Solicita uno nuevo.")
	case errors.Is(err, repository.ErrRegistroEnvios):
		return errRegistro(codes.ResourceExhausted, "reenvios_agotados",
			"Ya no se pueden enviar más códigos para este registro. Vuelve a registrarte más tarde.")
	case errors.Is(err, repository.ErrRegistroDemasiados):
		return errRegistro(codes.ResourceExhausted, "demasiadas_solicitudes",
			"Se hicieron demasiados registros con este correo. Inténtalo nuevamente en una hora.")
	case errors.Is(err, repository.ErrDNIExists):
		return errRegistro(codes.AlreadyExists, "dni_registrado",
			"Este DNI ya tiene una cuenta registrada. Inicia sesión con tus credenciales o comunícate con la institución.")
	case errors.Is(err, repository.ErrEmailExists):
		return errRegistro(codes.AlreadyExists, "correo_registrado",
			"Este correo ya tiene una cuenta registrada. Inicia sesión con tus credenciales.")
	case errors.Is(err, repository.ErrRegistroNoVerificado):
		return errRegistro(codes.FailedPrecondition, "no_verificado", "Primero verifica tu correo.")
	case errors.Is(err, repository.ErrRegistroCorreoNoCancelable):
		return errRegistro(codes.FailedPrecondition, "no_cancelable", "El registro no se puede cancelar.")
	case errors.Is(err, repository.ErrUsernameExists):
		return status.Error(codes.Aborted, "no se pudo crear la cuenta; inténtalo nuevamente")
	}
	return status.Error(codes.Internal, "no se pudo procesar el registro")
}

// errEnvio: el proveedor de correo falló (el registro no se guardó).
func errEnvio(ctx context.Context, err error) error {
	logx.WithContext(ctx).Errorf("registro con correo: no se pudo enviar el correo: %v", err)
	return errRegistro(codes.Unavailable, "envio_fallido",
		"No pudimos enviar el código a tu correo. Inténtalo nuevamente en unos minutos.")
}

// usernameSolicitante: identificador interno aleatorio (el login es por DNI).
func usernameSolicitante() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sol_" + hex.EncodeToString(b), nil
}

// eventoRegistro: auditoría de mejor esfuerzo (sin correo, DNI, código ni clave).
func eventoRegistro(ctx context.Context, svcCtx *svc.ServiceContext, usuarioID, nombre, resultado, motivo string) {
	if svcCtx.RegistroCorreoRepository == nil {
		return
	}
	if err := svcCtx.RegistroCorreoRepository.LogEvent(ctx, usuarioID, nombre, resultado, motivo); err != nil {
		logx.WithContext(ctx).Errorf("auditoría de registro (%s): no se pudo registrar el evento", nombre)
	}
}

// motivoRegistro: clave estable de un error registro_<clave> (auditoría).
func motivoRegistro(err error) string {
	msg := status.Convert(err).Message()
	if i := strings.Index(msg, ":"); strings.HasPrefix(msg, "registro_") && i > 0 {
		return msg[len("registro_"):i]
	}
	return status.Code(err).String()
}

func segundos(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	return int64(math.Ceil(d.Seconds()))
}

func largo(v string, n int) bool { return utf8.RuneCountInString(v) > n }
