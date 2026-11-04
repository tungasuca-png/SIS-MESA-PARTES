package logic

import (
	"context"
	"errors"
	"math"
	"net/mail"
	"strings"
	"time"

	"auth/auth"
	"auth/internal/repository"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LoginLogic) Login(in *auth.LoginRequest) (*auth.LoginResponse, error) {
	if in == nil || in.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "DNI y password son obligatorios")
	}
	// Solicitantes del registro público con el DNI todavía no verificado.
	if strings.TrimSpace(in.Email) != "" {
		return l.loginPorCorreo(in.Email, in.Password)
	}
	// Formato inválido -> InvalidArgument (no revela nada sobre existencia).
	// Un DNI con formato válido pero inexistente cae en credenciales inválidas.
	dni, err := normalizeDNI(in.Dni)
	if err != nil {
		return nil, err
	}

	user, err := l.svcCtx.UserRepository.FindByDNI(l.ctx, dni)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(in.Password))
			return nil, invalidCredentialsError()
		}
		return nil, status.Error(codes.Internal, "no se pudo procesar el login")
	}
	return l.autenticar(user, in.Password)
}

// autenticar: estado, bloqueo, clave y sesión; común al login por DNI y por
// correo (mismas reglas de intentos fallidos y bloqueo).
func (l *LoginLogic) autenticar(user *repository.User, password string) (*auth.LoginResponse, error) {
	if !user.Estado {
		return nil, invalidCredentialsError()
	}

	if user.LockedUntil.Valid && time.Now().Before(user.LockedUntil.Time) {
		return nil, accountLockedError(user.LockedUntil.Time)
	}
	if user.LockedUntil.Valid && !time.Now().Before(user.LockedUntil.Time) {
		if err := l.svcCtx.UserRepository.ClearExpiredLoginLock(l.ctx, user.ID); err != nil {
			return nil, status.Error(codes.Internal, "no se pudo procesar el login")
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		lockedUntil, err := l.svcCtx.UserRepository.RecordFailedLogin(l.ctx, user.ID)
		if err != nil {
			return nil, status.Error(codes.Internal, "no se pudo procesar el login")
		}
		// El intento que completa el límite ya informa el bloqueo.
		if lockedUntil.Valid && time.Now().Before(lockedUntil.Time) {
			return nil, accountLockedError(lockedUntil.Time)
		}
		return nil, invalidCredentialsError()
	}

	// Cuenta de un registro manual verificado cuyo perfil todavía no se creó.
	if l.svcCtx.RegistroCorreoRepository != nil {
		incompleto, err := l.svcCtx.RegistroCorreoRepository.RegistroIncompleto(l.ctx, user.ID)
		if err != nil {
			return nil, status.Error(codes.Internal, "no se pudo procesar el login")
		}
		if incompleto {
			return nil, status.Error(codes.PermissionDenied,
				"registro_incompleto: Tu registro no terminó. Vuelve a ingresar el código que te enviamos para completarlo.")
		}
	}

	roles, err := l.svcCtx.UserRepository.GetRoles(l.ctx, user.ID)
	if err != nil || len(roles) == 0 {
		return nil, status.Error(codes.Internal, "no se pudo obtener el rol del usuario")
	}
	if err := l.svcCtx.UserRepository.ResetLoginState(l.ctx, user.ID); err != nil {
		return nil, status.Error(codes.Internal, "no se pudo actualizar el estado del login")
	}

	return emitirSesion(l.ctx, l.svcCtx, user, roles[0])
}

// loginPorCorreo: SOLO cuentas SOLICITANTE (las del registro público inician
// sesión con su correo mientras Secretaría no verifica su DNI). Cualquier
// otra cuenta responde "credenciales inválidas" sin comparar su clave ni
// contar intentos: el login institucional sigue siendo exclusivamente por DNI.
func (l *LoginLogic) loginPorCorreo(email, password string) (*auth.LoginResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if a, err := mail.ParseAddress(email); err != nil || a.Address != email || len(email) > 254 {
		return nil, status.Error(codes.InvalidArgument, "el correo no tiene un formato válido")
	}
	user, err := l.svcCtx.UserRepository.FindByEmail(l.ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, l.sinCuenta(email, password)
		}
		return nil, status.Error(codes.Internal, "no se pudo procesar el login")
	}
	roles, err := l.svcCtx.UserRepository.GetRoles(l.ctx, user.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo procesar el login")
	}
	if len(roles) != 1 || roles[0] != repository.RolSolicitante {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
		return nil, invalidCredentialsError()
	}
	return l.autenticar(user, password)
}

// sinCuenta: el correo no tiene cuenta. Si tiene un registro manual pendiente
// y la clave es la de ese registro, avisa que falta verificar el correo (solo
// quien conoce la clave lo sabe); si no, credenciales inválidas. Siempre se
// calcula un bcrypt, con o sin registro (mismo tiempo de respuesta).
func (l *LoginLogic) sinCuenta(email, password string) error {
	hash := dummyPasswordHash
	pendiente := false
	if l.svcCtx.RegistroCorreoRepository != nil {
		h, err := l.svcCtx.RegistroCorreoRepository.ClavePendientePorCorreo(l.ctx, email)
		switch {
		case err == nil:
			hash, pendiente = h, true
		case !errors.Is(err, repository.ErrRegistroNoDisponible):
			return status.Error(codes.Internal, "no se pudo procesar el login")
		}
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil && pendiente {
		return status.Error(codes.PermissionDenied,
			"correo_no_verificado: Tu correo todavía no está verificado. Ingresa el código que te enviamos o regístrate de nuevo para recibir otro.")
	}
	return invalidCredentialsError()
}

func invalidCredentialsError() error {
	return status.Error(codes.Unauthenticated, "credenciales inválidas")
}

// accountLockedError informa el bloqueo temporal (5 intentos fallidos) y los
// minutos que faltan, redondeados hacia arriba. ResourceExhausted -> HTTP 429
// en el Gateway. Decisión solicitada: se prefiere avisar del bloqueo, aunque
// así se revela que el DNI corresponde a una cuenta (solo tras 5 fallos).
func accountLockedError(until time.Time) error {
	minutos := int(math.Ceil(time.Until(until).Minutes()))
	if minutos < 1 {
		minutos = 1
	}
	return status.Errorf(codes.ResourceExhausted,
		"account_locked: cuenta bloqueada temporalmente por demasiados intentos fallidos; intenta de nuevo en %d minutos", minutos)
}
