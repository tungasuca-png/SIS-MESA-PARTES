// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"documentos/documentosclient"

	"gateway/internal/reportes"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReporteDocumentosLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReporteDocumentosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReporteDocumentosLogic {
	return &ReporteDocumentosLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ReporteDocumentos: transporte. Documentos Service exige "reportes.view",
// obtiene de Expedientes el alcance (sin bajas lógicas) y agrega en SQL.
func (l *ReporteDocumentosLogic) ReporteDocumentos(req *types.ReportePeriodoRequest) (resp *types.ReporteDocumentosResponse, err error) {
	r, err := consultarDocumentos(l.ctx, l.svcCtx, req.Authorization, req.Desde, req.Hasta, req.Tipo, req.Granularidad, false)
	if err != nil {
		return nil, err
	}
	return &types.ReporteDocumentosResponse{
		Total: r.Total, PorTipoDocumento: conteosDocDTO(r.PorTipoDocumento), PorTipoTramite: conteosDocDTO(r.PorTipoTramite),
		Serie: conteosDocDTO(r.Serie), Granularidad: r.Granularidad, Alcance: r.Alcance, AreaAlcance: r.AreaAlcance, Generado: r.Generado,
	}, nil
}

func consultarDocumentos(ctx context.Context, svcCtx *svc.ServiceContext, authorization, desde, hasta, tipo, granularidad string, registros bool) (*documentosclient.ReporteDocumentosResponse, error) {
	return svcCtx.DocumentosClient.ReporteDocumentos(withAuthorization(ctx, authorization), &documentosclient.ReporteDocumentosRequest{
		Desde: desde, Hasta: hasta, TipoTramite: tipo, Granularidad: granularidad, IncluirRegistros: registros,
	})
}

func conteosDocDTO(in []*documentosclient.ReporteDocConteo) []types.ReporteConteoDTO {
	out := make([]types.ReporteConteoDTO, 0, len(in))
	for _, c := range in {
		out = append(out, types.ReporteConteoDTO{Clave: c.Clave, Cantidad: c.Cantidad})
	}
	return out
}

func conteosDoc(in []*documentosclient.ReporteDocConteo) []reportes.Conteo {
	out := make([]reportes.Conteo, 0, len(in))
	for _, c := range in {
		out = append(out, reportes.Conteo{Clave: c.Clave, Cantidad: int(c.Cantidad)})
	}
	return out
}
