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

type ReporteExpedientesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReporteExpedientesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReporteExpedientesLogic {
	return &ReporteExpedientesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ReporteExpedientes: transporte puro. Expedientes Service valida
// "reportes.view", los filtros y el alcance por rol, y agrega en SQL.
// Fase 2: "granularidad" (opcional) agrega la serie; las tablas cruzadas
// siempre vienen. Sin granularidad, los campos de la Fase 1 son idénticos.
func (l *ReporteExpedientesLogic) ReporteExpedientes(req *types.ReporteExpedientesRequest) (resp *types.ReporteExpedientesResponse, err error) {
	r, err := consultarReporte(l.ctx, l.svcCtx, req.Authorization, filtrosReporte{
		Desde: req.Desde, Hasta: req.Hasta, Estado: req.Estado, Tipo: req.Tipo, Prioridad: req.Prioridad, Area: req.Area,
		Granularidad: req.Granularidad,
	})
	if err != nil {
		return nil, err
	}
	return &types.ReporteExpedientesResponse{
		Total:                   r.Total,
		PorEstado:               toConteosDTO(r.PorEstado),
		PorTipo:                 toConteosDTO(r.PorTipo),
		PorPrioridad:            toConteosDTO(r.PorPrioridad),
		PorArea:                 toConteosDTO(r.PorArea),
		PorMes:                  toConteosDTO(r.PorMes),
		Pendientes:              r.Pendientes,
		PendientesPorAntiguedad: toConteosDTO(r.PendientesPorAntiguedad),
		Alcance:                 r.Alcance,
		AreaAlcance:             r.AreaAlcance,
		Generado:                r.Generado,
		Serie:                   toConteosDTO(r.Serie),
		Granularidad:            r.Granularidad,
		AreaEstado:              toCrucesDTO(r.AreaEstado),
		TipoEstado:              toCrucesDTO(r.TipoEstado),
		SerieEstado:             toCrucesDTO(r.SerieEstado),
	}, nil
}

type filtrosReporte struct {
	Desde, Hasta, Estado, Tipo, Prioridad, Area, Granularidad string
	IncluirRegistros                                          bool
}

func consultarReporte(ctx context.Context, svcCtx *svc.ServiceContext, authorization string, f filtrosReporte) (*expedientesclient.ReporteExpedientesResponse, error) {
	return svcCtx.ExpedientesClient.ReporteExpedientes(withAuthorization(ctx, authorization), &expedientesclient.ReporteExpedientesRequest{
		Desde: f.Desde, Hasta: f.Hasta, Estado: f.Estado, Tipo: f.Tipo, Prioridad: f.Prioridad, Area: f.Area,
		Granularidad: f.Granularidad, IncluirRegistros: f.IncluirRegistros,
	})
}

func toConteosDTO(in []*expedientesclient.ReporteConteo) []types.ReporteConteoDTO {
	out := make([]types.ReporteConteoDTO, 0, len(in))
	for _, c := range in {
		out = append(out, types.ReporteConteoDTO{Clave: c.Clave, Cantidad: c.Cantidad})
	}
	return out
}

func toCrucesDTO(in []*expedientesclient.ReporteCruce) []types.ReporteCruceDTO {
	out := make([]types.ReporteCruceDTO, 0, len(in))
	for _, c := range in {
		out = append(out, types.ReporteCruceDTO{Fila: c.Fila, Columna: c.Columna, Cantidad: c.Cantidad})
	}
	return out
}
