// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"strings"

	"derivaciones/derivacionesclient"
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

// RechazarExpediente (Etapa 4): mismo principio que DerivarExpediente —
// primero se mueve el estado y el área (autoritativo, Expedientes Service
// ya valida que sea F4 y que el actor sea dueño del área actual), y solo
// si eso tuvo éxito se registra el historial en Derivaciones Service.
func (l *RechazarExpedienteLogic) RechazarExpediente(req *types.RechazarExpedienteRequest) (resp *types.RechazarExpedienteResponse, err error) {
	if strings.TrimSpace(req.Motivo) == "" {
		// Se valida acá (antes de mover nada) para no dejar el expediente ya
		// rechazado y solo entonces fallar por falta de motivo — el mismo
		// requisito que ya exige CrearDerivacion, adelantado.
		return nil, status.Error(codes.InvalidArgument, "el motivo es obligatorio")
	}

	ctx := withAuthorization(l.ctx, req.Authorization)

	before, err := l.svcCtx.ExpedientesClient.GetExpediente(ctx, &expedientesclient.GetExpedienteRequest{Id: req.Id})
	if err != nil {
		return nil, err
	}
	origen := before.Expediente.AreaActual

	rechazado, err := l.svcCtx.ExpedientesClient.RechazarExpediente(ctx, &expedientesclient.RechazarExpedienteRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}

	created, err := l.svcCtx.DerivacionesClient.CrearDerivacion(ctx, &derivacionesclient.CrearDerivacionRequest{
		ExpedienteId: req.Id,
		Tipo:         "DERIVACION",
		Origen:       origen,
		Destino:      "SECRETARIA",
		Motivo:       req.Motivo,
	})
	if err != nil {
		return nil, status.Errorf(
			status.Code(err),
			"el expediente se rechazó y devolvió a Secretaría, pero no se pudo registrar el historial: %v",
			err,
		)
	}

	return &types.RechazarExpedienteResponse{
		Expediente: toExpedienteDTO(rechazado.Expediente),
		Derivacion: toDerivacionDTO(created.Derivacion),
	}, nil
}
