package logic

import (
	"context"
	"errors"
	"math"
	"time"

	"auth/auth"
	"auth/internal/repository"
	"auth/internal/security"
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

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)); err != nil {
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

	roles, err := l.svcCtx.UserRepository.GetRoles(l.ctx, user.ID)
	if err != nil || len(roles) == 0 {
		return nil, status.Error(codes.Internal, "no se pudo obtener el rol del usuario")
	}
	if err := l.svcCtx.UserRepository.ResetLoginState(l.ctx, user.ID); err != nil {
		return nil, status.Error(codes.Internal, "no se pudo actualizar el estado del login")
	}

	accessToken, expiresIn, err := l.svcCtx.JWTManager.GenerateAccessToken(
		user.ID,
		user.Username,
		roles[0],
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo generar el token")
	}
	refreshToken, refreshTokenHash, err := security.GenerateRefreshToken()
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo generar el refresh token")
	}
	if l.svcCtx.Config.RefreshTokenDuration <= 0 {
		return nil, status.Error(codes.Internal, "la duración del refresh token no está configurada")
	}
	if err := l.svcCtx.RefreshTokenRepository.Create(
		l.ctx,
		user.ID,
		refreshTokenHash,
		time.Now().Add(l.svcCtx.Config.RefreshTokenDuration),
	); err != nil {
		return nil, status.Error(codes.Internal, "no se pudo guardar el refresh token")
	}

	return &auth.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(expiresIn / time.Second),
		UserId:       user.ID,
		Username:     user.Username,
		Email:        user.Email,
		Role:         roles[0],
	}, nil
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
