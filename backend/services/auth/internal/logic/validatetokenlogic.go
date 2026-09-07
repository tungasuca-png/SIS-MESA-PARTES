package logic

import (
	"context"
	"strings"

	"auth/auth"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ValidateTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateTokenLogic {
	return &ValidateTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ValidateTokenLogic) ValidateToken(in *auth.ValidateTokenRequest) (*auth.ValidateTokenResponse, error) {
	if in == nil || strings.TrimSpace(in.AccessToken) == "" {
		return nil, status.Error(codes.Unauthenticated, "token de acceso inválido")
	}

	claims, err := l.svcCtx.JWTManager.ValidateAccessToken(in.AccessToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "token de acceso inválido")
	}

	return &auth.ValidateTokenResponse{
		Valid:    true,
		UserId:   claims.Subject,
		Username: claims.Username,
		Role:     claims.Role,
	}, nil
}
