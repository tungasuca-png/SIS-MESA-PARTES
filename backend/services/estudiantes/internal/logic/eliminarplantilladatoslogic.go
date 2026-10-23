package logic

import (
	"context"

	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// EliminarPlantillaDatosLogic: ver plantillasdatos.go.
type EliminarPlantillaDatosLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEliminarPlantillaDatosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EliminarPlantillaDatosLogic {
	return &EliminarPlantillaDatosLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}
