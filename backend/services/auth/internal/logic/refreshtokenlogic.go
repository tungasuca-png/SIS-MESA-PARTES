package logic

import (
	"context"
	"errors"
	"time"

	"auth/auth"
	"auth/internal/repository"
	"auth/internal/security"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RefreshTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RefreshTokenLogic) RefreshToken(in *auth.RefreshTokenRequest) (*auth.RefreshTokenResponse, error) {
	if in == nil || in.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "el refresh token es obligatorio")
	}

	tokenHash := security.HashRefreshToken(in.RefreshToken)
	current, err := l.svcCtx.RefreshTokenRepository.FindByHash(l.ctx, tokenHash)
	if err != nil {
		return nil, refreshTokenError(err)
	}
	if current.RevokedAt.Valid || !time.Now().Before(current.ExpiresAt) {
		return nil, refreshTokenError(repository.ErrRefreshTokenRevoked)
	}

	user, err := l.svcCtx.UserRepository.FindByID(l.ctx, current.UserID)
	if err != nil || !user.Estado {
		return nil, refreshTokenError(repository.ErrUserNotFound)
	}
	roles, err := l.svcCtx.UserRepository.GetRoles(l.ctx, user.ID)
	if err != nil || len(roles) == 0 {
		return nil, status.Error(codes.Internal, "no se pudo obtener el rol del usuario")
	}
	if l.svcCtx.Config.RefreshTokenDuration <= 0 {
		return nil, status.Error(codes.Internal, "la duración del refresh token no está configurada")
	}

	accessToken, expiresIn, err := l.svcCtx.JWTManager.GenerateAccessToken(user.ID, user.Username, roles[0])
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo generar el token")
	}
	newRefreshToken, newRefreshTokenHash, err := security.GenerateRefreshToken()
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo generar el refresh token")
	}
	if _, err := l.svcCtx.RefreshTokenRepository.Rotate(
		l.ctx,
		tokenHash,
		newRefreshTokenHash,
		time.Now().Add(l.svcCtx.Config.RefreshTokenDuration),
	); err != nil {
		return nil, refreshTokenError(err)
	}

	return &auth.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(expiresIn / time.Second),
	}, nil
}

func refreshTokenError(err error) error {
	switch {
	case errors.Is(err, repository.ErrRefreshTokenNotFound),
		errors.Is(err, repository.ErrRefreshTokenRevoked),
		errors.Is(err, repository.ErrRefreshTokenExpired),
		errors.Is(err, repository.ErrUserNotFound):
		return status.Error(codes.Unauthenticated, "refresh token inválido")
	default:
		return status.Error(codes.Internal, "no se pudo procesar el refresh token")
	}
}
