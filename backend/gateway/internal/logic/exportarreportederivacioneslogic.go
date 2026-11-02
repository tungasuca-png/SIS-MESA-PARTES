// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"strings"

	"gateway/internal/reportes"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExportarReporteDerivacionesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExportarReporteDerivacionesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportarReporteDerivacionesLogic {
	return &ExportarReporteDerivacionesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ExportarReporteDerivaciones: el MISMO reporte (mismos filtros, permiso y
// alcance) en xlsx o csv.
func (l *ExportarReporteDerivacionesLogic) ExportarReporteDerivaciones(req *types.ExportarReportePeriodoRequest) (resp *types.ArchivoReporteResponse, err error) {
	formato, err := formatoReporte(req.Formato)
	if err != nil {
		return nil, err
	}
	r, err := consultarDerivaciones(l.ctx, l.svcCtx, req.Authorization, req.Desde, req.Hasta, req.Tipo, req.Granularidad, false)
	if err != nil {
		return nil, err
	}
	flujo := make([]reportes.Cruce, 0, len(r.Flujo))
	for _, f := range r.Flujo {
		flujo = append(flujo, reportes.Cruce{Fila: f.Origen, Columna: f.Destino, Cantidad: int(f.Cantidad)})
	}
	generado := generadoLima(r.Generado)
	doc := reportes.DocumentoDerivaciones(reportes.DatosDerivaciones{
		Total: int(r.Total), SinClasificar: int(r.SinClasificar),
		PorDestino: conteosDeriv(r.PorDestino), PorOrigen: conteosDeriv(r.PorOrigen), PorTipoTramite: conteosDeriv(r.PorTipoTramite),
		Serie: conteosDeriv(r.Serie), Otros: conteosDeriv(r.OtrosMovimientos), Flujo: flujo,
		Granularidad: r.Granularidad, Alcance: r.Alcance, AreaAlcance: r.AreaAlcance, Generado: generado,
	}, filtrosPeriodo(req.Desde, req.Hasta, req.Tipo, r.Granularidad))
	return archivoReporte(l.Logger, doc, reportes.NombreArchivoDe("derivaciones", generado, formato), formato)
}

func filtrosPeriodo(desde, hasta, tipo, granularidad string) reportes.FiltrosPeriodo {
	return reportes.FiltrosPeriodo{
		Desde: strings.TrimSpace(desde), Hasta: strings.TrimSpace(hasta),
		Tipo: strings.ToUpper(strings.TrimSpace(tipo)), Granularidad: granularidad,
	}
}
