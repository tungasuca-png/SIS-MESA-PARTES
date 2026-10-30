package logic

import (
	"context"

	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// GuardarPlantillaDatosLogic: ver plantillasdatos.go.
type GuardarPlantillaDatosLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGuardarPlantillaDatosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GuardarPlantillaDatosLogic {
	return &GuardarPlantillaDatosLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}
