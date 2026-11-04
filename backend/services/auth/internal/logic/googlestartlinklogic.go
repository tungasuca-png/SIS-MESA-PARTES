package logic

import (
	"context"
	"errors"
	"time"

	"auth/auth"
	"auth/internal/interceptor"
	"auth/internal/repository"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GoogleStartLinkLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogleStartLinkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GoogleStartLinkLogic {
	return &GoogleStartLinkLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GoogleStartLink: un SOLICITANTE autenticado vuelve a demostrar el control de
// su cuenta con su clave actual (mismo conteo de intentos y bloqueo que el
// login) y recibe un código VINCULACION de un solo uso. La cuenta solo se
// identifica por el JWT, nunca por datos de la solicitud (sin IDOR).
func (l *GoogleStartLinkLogic) GoogleStartLink(in *auth.GoogleStartLinkRequest) (*auth.GoogleStartLinkResponse, error) {
	usuarioID, ok := interceptor.UserIDFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if !l.svcCtx.Google.Configurado() {
		return nil, errNoConfigurado()
	}
	if in.GetPassword() == "" {
		return nil, errGoogle(codes.InvalidArgument, "clave_requerida", "Ingresa tu clave actual para vincular Google.")
	}
	user, _, err := cuentaSolicitanteDisponible(l.ctx, l.svcCtx, usuarioID)
	if err != nil {
		evento(l.ctx, l.svcCtx, usuarioID, "google_vinculacion_inicio", "RECHAZADO", motivoDe(err))
		return nil, err
	}
	if _, err := l.svcCtx.GoogleRepository.FindIdentityByUser(l.ctx, usuarioID); err == nil {
		return nil, errGoogle(codes.AlreadyExists, "cuenta_ya_vinculada", "Tu cuenta ya tiene una cuenta de Google vinculada.")
	} else if !errors.Is(err, repository.ErrIdentidadNoEncontrada) {
		return nil, status.Error(codes.Internal, "no se pudo consultar la vinculación")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.GetPassword())); err != nil {
		lockedUntil, err := l.svcCtx.UserRepository.RecordFailedLogin(l.ctx, user.ID)
		if err != nil {
			return nil, status.Error(codes.Internal, "no se pudo verificar la clave")
		}
		evento(l.ctx, l.svcCtx, usuarioID, "google_vinculacion_inicio", "RECHAZADO", "clave_incorrecta")
		if lockedUntil.Valid && time.Now().Before(lockedUntil.Time) {
			return nil, accountLockedError(lockedUntil.Time)
		}
		// 400 (no 401): un 401 cerraría la sesión en el frontend.
		return nil, errGoogle(codes.InvalidArgument, "clave_incorrecta", "La clave no es correcta.")
	}
	if err := l.svcCtx.UserRepository.ResetLoginState(l.ctx, user.ID); err != nil {
		return nil, status.Error(codes.Internal, "no se pudo verificar la clave")
	}
	codigo, err := nuevoCodigo(l.ctx, l.svcCtx, repository.NuevoCodigo{Purpose: repository.PropositoVinculacion, UsuarioID: usuarioID}, duracionCodigoVinculacion)
	if err != nil {
		return nil, err
	}
	evento(l.ctx, l.svcCtx, usuarioID, "google_vinculacion_inicio", "OK", "")
	return &auth.GoogleStartLinkResponse{LinkCode: codigo, ExpiresIn: int64(duracionCodigoVinculacion / time.Second)}, nil
}
