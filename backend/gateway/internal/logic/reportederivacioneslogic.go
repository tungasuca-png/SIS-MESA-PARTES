// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"derivaciones/derivacionesclient"

	"gateway/internal/reportes"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReporteDerivacionesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReporteDerivacionesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReporteDerivacionesLogic {
	return &ReporteDerivacionesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ReporteDerivaciones: transporte. Derivaciones Service exige
// "reportes.view", obtiene de Expedientes el alcance (sin bajas lógicas),
// agrega en SQL y normaliza origen/destino (sin textos libres).
func (l *ReporteDerivacionesLogic) ReporteDerivaciones(req *types.ReportePeriodoRequest) (resp *types.ReporteDerivacionesResponse, err error) {
	r, err := consultarDerivaciones(l.ctx, l.svcCtx, req.Authorization, req.Desde, req.Hasta, req.Tipo, req.Granularidad, false)
	if err != nil {
		return nil, err
	}
	flujo := make([]types.ReporteFlujoDTO, 0, len(r.Flujo))
	for _, f := range r.Flujo {
		flujo = append(flujo, types.ReporteFlujoDTO{Origen: f.Origen, Destino: f.Destino, Cantidad: f.Cantidad})
	}
	return &types.ReporteDerivacionesResponse{
		Total: r.Total, PorDestino: conteosDerivDTO(r.PorDestino), PorOrigen: conteosDerivDTO(r.PorOrigen), Flujo: flujo,
		PorTipoTramite: conteosDerivDTO(r.PorTipoTramite), Serie: conteosDerivDTO(r.Serie), Granularidad: r.Granularidad,
		OtrosMovimientos: conteosDerivDTO(r.OtrosMovimientos), SinClasificar: r.SinClasificar,
		Alcance: r.Alcance, AreaAlcance: r.AreaAlcance, Generado: r.Generado,
	}, nil
}

func consultarDerivaciones(ctx context.Context, svcCtx *svc.ServiceContext, authorization, desde, hasta, tipo, granularidad string, registros bool) (*derivacionesclient.ReporteDerivacionesResponse, error) {
	return svcCtx.DerivacionesClient.ReporteDerivaciones(withAuthorization(ctx, authorization), &derivacionesclient.ReporteDerivacionesRequest{
		Desde: desde, Hasta: hasta, TipoTramite: tipo, Granularidad: granularidad, IncluirRegistros: registros,
	})
}

func conteosDerivDTO(in []*derivacionesclient.ReporteDerivConteo) []types.ReporteConteoDTO {
	out := make([]types.ReporteConteoDTO, 0, len(in))
	for _, c := range in {
		out = append(out, types.ReporteConteoDTO{Clave: c.Clave, Cantidad: c.Cantidad})
	}
	return out
}

func conteosDeriv(in []*derivacionesclient.ReporteDerivConteo) []reportes.Conteo {
	out := make([]reportes.Conteo, 0, len(in))
	for _, c := range in {
		out = append(out, reportes.Conteo{Clave: c.Clave, Cantidad: int(c.Cantidad)})
	}
	return out
}
