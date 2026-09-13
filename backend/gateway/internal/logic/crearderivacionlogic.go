// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"derivaciones/derivacionesclient"
	"expedientes/expedientesclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CrearDerivacionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCrearDerivacionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CrearDerivacionLogic {
	return &CrearDerivacionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CrearDerivacionLogic) CrearDerivacion(req *types.CrearDerivacionRequest) (resp *types.CrearDerivacionResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	// Derivaciones Service no conoce area_actual (es un microservicio
	// aislado, sin FK hacia Expedientes — ver docs/etapa-12-especificacion-
	// funcional.md sección 20), así que no puede validar por sí solo si
	// quien registra la derivación tiene autorización sobre ESE expediente.
	// En vez de acoplar Derivaciones Service directamente a Expedientes
	// Service, se reutiliza acá la misma validación que ya aplica
	// GetExpediente (Expedientes Service exige pertenecer al área actual,
	// salvo ADMIN/SECRETARIA) — este es el patrón ya establecido en el
	// proyecto de componer en el Gateway en vez de acoplar microservicios
	// entre sí (mismo motivo por el que Documentos Service no llama a
	// Expedientes Service).
	if _, err := l.svcCtx.ExpedientesClient.GetExpediente(ctx, &expedientesclient.GetExpedienteRequest{
		Id: req.ExpedienteId,
	}); err != nil {
		return nil, err
	}

	created, err := l.svcCtx.DerivacionesClient.CrearDerivacion(ctx, &derivacionesclient.CrearDerivacionRequest{
		ExpedienteId: req.ExpedienteId,
		Tipo:         req.Tipo,
		Origen:       req.Origen,
		Destino:      req.Destino,
		Motivo:       req.Motivo,
		Condicion:    req.Condicion,
	})
	if err != nil {
		return nil, err
	}

	return &types.CrearDerivacionResponse{Derivacion: toDerivacionDTO(created.Derivacion)}, nil
}
