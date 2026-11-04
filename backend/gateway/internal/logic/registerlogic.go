// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Register: el autorregistro público quedó deshabilitado (el handler responde
// 410 Gone sin llegar aquí). Se conserva la firma generada por goctl, pero
// ya no llama a Auth.Register.
func (l *RegisterLogic) Register(_ *types.RegisterRequest) (resp *types.RegisterResponse, err error) {
	return nil, status.Error(codes.Unimplemented, "el registro público fue deshabilitado")
}
