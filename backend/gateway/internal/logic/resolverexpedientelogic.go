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

type ResolverExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResolverExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolverExpedienteLogic {
	return &ResolverExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ResolverExpediente (Etapa 4): "el cierre final ocurre cuando Secretaría
// deja disponible el documento" — se verifica que el documento final
// (PROVEIDO) exista en Documentos Service ANTES de cerrar el expediente
// en Expedientes Service (que no tiene cómo verificarlo, no conoce
// Documentos Service — mismo límite ya documentado en etapas anteriores).
func (l *ResolverExpedienteLogic) ResolverExpediente(req *types.ResolverExpedienteRequest) (resp *types.ResolverExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	// Documentos Service solo conoce el UUID real del expediente (no el
	// código legible EXP-YYYY-NNNNNN que también acepta Expedientes
	// Service) — se resuelve primero.
	before, err := l.svcCtx.ExpedientesClient.GetExpediente(ctx, &expedientesclient.GetExpedienteRequest{Id: req.Id})
	if err != nil {
		return nil, err
	}
	if err := requireDocumentoProveido(ctx, l.svcCtx, before.Expediente.Id); err != nil {
		return nil, err
	}

	resuelto, err := l.svcCtx.ExpedientesClient.ResolverExpediente(ctx, &expedientesclient.ResolverExpedienteRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.ResolverExpedienteResponse{Expediente: toExpedienteDTO(resuelto.Expediente)}, nil
}
