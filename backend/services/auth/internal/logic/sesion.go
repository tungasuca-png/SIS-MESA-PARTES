package logic

import (
	"context"
	"time"

	"auth/auth"
	"auth/internal/repository"
	"auth/internal/security"
	"auth/internal/svc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// emitirSesion: access token (JWT propio) + refresh token rotativo guardado
// como hash. Lo usan Login (DNI y clave) y el acceso con Google: un único
// sistema de sesiones.
func emitirSesion(ctx context.Context, svcCtx *svc.ServiceContext, user *repository.User, role string) (*auth.LoginResponse, error) {
	accessToken, expiresIn, err := svcCtx.JWTManager.GenerateAccessToken(user.ID, user.Username, role)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo generar el token")
	}
	refreshToken, refreshTokenHash, err := security.GenerateRefreshToken()
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo generar el refresh token")
	}
	if svcCtx.Config.RefreshTokenDuration <= 0 {
		return nil, status.Error(codes.Internal, "la duración del refresh token no está configurada")
	}
	if err := svcCtx.RefreshTokenRepository.Create(ctx, user.ID, refreshTokenHash, time.Now().Add(svcCtx.Config.RefreshTokenDuration)); err != nil {
		return nil, status.Error(codes.Internal, "no se pudo guardar el refresh token")
	}
	return &auth.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(expiresIn / time.Second),
		UserId:       user.ID,
		Username:     user.Username,
		Email:        user.Email,
		Role:         role,
	}, nil
}
