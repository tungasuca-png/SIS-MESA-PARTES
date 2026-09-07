package logic

import (
	"context"
	"errors"
	"strings"

	"auth/auth"
	"auth/internal/interceptor"
	"auth/internal/repository"
	"auth/internal/security"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LogoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LogoutLogic) Logout(in *auth.LogoutRequest) (*auth.LogoutResponse, error) {
	userID, ok := interceptor.UserIDFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if in == nil || strings.TrimSpace(in.RefreshToken) == "" {
		return nil, status.Error(codes.InvalidArgument, "el refresh token es obligatorio")
	}

	refreshToken := strings.TrimSpace(in.RefreshToken)
	current, err := l.svcCtx.RefreshTokenRepository.FindByHash(
		l.ctx,
		security.HashRefreshToken(refreshToken),
	)
	if err != nil {
		return nil, logoutTokenError(err)
	}
	if current.UserID != userID || current.RevokedAt.Valid {
		return nil, status.Error(codes.Unauthenticated, "refresh token inválido")
	}
	if err := l.svcCtx.RefreshTokenRepository.Revoke(l.ctx, current.ID); err != nil {
		return nil, status.Error(codes.Internal, "no se pudo cerrar la sesión")
	}

	return &auth.LogoutResponse{
		Success: true,
		Message: "sesión cerrada correctamente",
	}, nil
}

func logoutTokenError(err error) error {
	if errors.Is(err, repository.ErrRefreshTokenNotFound) || errors.Is(err, repository.ErrRefreshTokenRevoked) {
		return status.Error(codes.Unauthenticated, "refresh token inválido")
	}
	return status.Error(codes.Internal, "no se pudo cerrar la sesión")
}
