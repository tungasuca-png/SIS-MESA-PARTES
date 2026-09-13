// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"expedientes/expedientesclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CorregirExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCorregirExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CorregirExpedienteLogic {
	return &CorregirExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CorregirExpediente (Etapa 4): no cambia de área (sigue en SECRETARIA,
// donde quedó tras el rechazo) — no hay nada que componer con
// Derivaciones Service; el rechazo original ya quedó en el historial con
// su motivo, quién y cuándo (ver RechazarExpediente).
func (l *CorregirExpedienteLogic) CorregirExpediente(req *types.CorregirExpedienteRequest) (resp *types.CorregirExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	corregido, err := l.svcCtx.ExpedientesClient.CorregirExpediente(ctx, &expedientesclient.CorregirExpedienteRequest{
		Id:          req.Id,
		Descripcion: req.Descripcion,
	})
	if err != nil {
		return nil, err
	}

	return &types.CorregirExpedienteResponse{Expediente: toExpedienteDTO(corregido.Expediente)}, nil
}
