// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"strings"

	"expedientes/expedientesclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RechazarExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRechazarExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RechazarExpedienteLogic {
	return &RechazarExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// RechazarExpediente (A + B): una sola llamada; Expedientes cambia estado y
// área y registra el movimiento RECHAZO en la misma transacción.
func (l *RechazarExpedienteLogic) RechazarExpediente(req *types.RechazarExpedienteRequest) (resp *types.RechazarExpedienteResponse, err error) {
	if strings.TrimSpace(req.Motivo) == "" {
		return nil, status.Error(codes.InvalidArgument, "el motivo es obligatorio")
	}
	ctx := withAuthorization(l.ctx, req.Authorization)
	rechazado, err := l.svcCtx.ExpedientesClient.RechazarExpediente(ctx, &expedientesclient.RechazarExpedienteRequest{
		Id:                 req.Id,
		Motivo:             req.Motivo,
		AreaOrigenEsperada: req.AreaOrigenEsperada,
		IdempotencyKey:     req.IdempotencyKey,
	})
	if err != nil {
		return nil, errorMovimiento(err)
	}
	return &types.RechazarExpedienteResponse{
		Expediente: toExpedienteDTO(rechazado.Expediente),
		Movimiento: toMovimientoDTO(rechazado.Movimiento),
		Repetido:   rechazado.Repetido,
		Derivacion: derivacionDeMovimiento(rechazado.Movimiento),
	}, nil
}
