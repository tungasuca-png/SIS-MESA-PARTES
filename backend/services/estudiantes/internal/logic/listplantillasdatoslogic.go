package logic

import (
	"context"

	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// ListPlantillasDatosLogic: ver plantillasdatos.go.
type ListPlantillasDatosLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPlantillasDatosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlantillasDatosLogic {
	return &ListPlantillasDatosLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}
