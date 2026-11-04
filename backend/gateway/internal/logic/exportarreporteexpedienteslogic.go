// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"strings"

	"expedientes/expedientesclient"

	"gateway/internal/reportes"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ExportarReporteExpedientesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExportarReporteExpedientesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportarReporteExpedientesLogic {
	return &ExportarReporteExpedientesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

const (
	vistaResumen     = "RESUMEN"
	vistaExpedientes = "EXPEDIENTES"
)

// ExportarReporteExpedientes genera el archivo del MISMO reporte (mismos
// filtros, mismo permiso y alcance: la consulta la valida Expedientes
// Service). vista RESUMEN (por defecto) = archivo de la Fase 1; vista
// EXPEDIENTES = serie por granularidad y tablas cruzadas (Fase 2).
func (l *ExportarReporteExpedientesLogic) ExportarReporteExpedientes(req *types.ExportarReporteExpedientesRequest) (resp *types.ArchivoReporteResponse, err error) {
	formato, err := formatoReporte(req.Formato)
	if err != nil {
		return nil, err
	}
	vista := strings.ToUpper(strings.TrimSpace(req.Vista))
	if vista == "" {
		vista = vistaResumen
	}
	if vista != vistaResumen && vista != vistaExpedientes {
		return nil, status.Error(codes.InvalidArgument, "validation_error: vista no admitida: use RESUMEN o EXPEDIENTES")
	}
	f := filtrosReporte{Desde: req.Desde, Hasta: req.Hasta, Estado: req.Estado, Tipo: req.Tipo, Prioridad: req.Prioridad, Area: req.Area}
	if vista == vistaExpedientes {
		f.Granularidad = req.Granularidad
		if strings.TrimSpace(f.Granularidad) == "" {
			f.Granularidad = "MES"
		}
	}
	r, err := consultarReporte(l.ctx, l.svcCtx, req.Authorization, f)
	if err != nil {
		return nil, err
	}
	datos := datosReporte(r)
	filtros := filtrosImpresos(f)
	doc := reportes.DocumentoResumen(datos, filtros)
	nombre := reportes.NombreArchivo(datos, formato)
	if vista == vistaExpedientes {
		doc = reportes.DocumentoExpedientes(reportes.DatosExpedientes{
			Base: datos, Serie: conteosExp(r.Serie), Granularidad: r.Granularidad,
			AreaEstado: crucesExp(r.AreaEstado), TipoEstado: crucesExp(r.TipoEstado), SerieEstado: crucesExp(r.SerieEstado),
		}, filtros)
		nombre = reportes.NombreArchivoDe("expedientes-areas", datos.Generado, formato)
	}
	return archivoReporte(l.Logger, doc, nombre, formato)
}

// filtrosImpresos: los filtros NORMALIZADOS (igual que los aplicó el servicio).
func filtrosImpresos(f filtrosReporte) reportes.Filtros {
	return reportes.Filtros{
		Desde: strings.TrimSpace(f.Desde), Hasta: strings.TrimSpace(f.Hasta),
		Estado: strings.ToUpper(strings.TrimSpace(f.Estado)), Tipo: strings.ToUpper(strings.TrimSpace(f.Tipo)),
		Prioridad: strings.ToUpper(strings.TrimSpace(f.Prioridad)), Area: strings.ToUpper(strings.TrimSpace(f.Area)),
	}
}

func conteosExp(in []*expedientesclient.ReporteConteo) []reportes.Conteo {
	out := make([]reportes.Conteo, 0, len(in))
	for _, c := range in {
		out = append(out, reportes.Conteo{Clave: c.Clave, Cantidad: int(c.Cantidad)})
	}
	return out
}

func crucesExp(in []*expedientesclient.ReporteCruce) []reportes.Cruce {
	out := make([]reportes.Cruce, 0, len(in))
	for _, c := range in {
		out = append(out, reportes.Cruce{Fila: c.Fila, Columna: c.Columna, Cantidad: int(c.Cantidad)})
	}
	return out
}

func datosReporte(r *expedientesclient.ReporteExpedientesResponse) reportes.Datos {
	return reportes.Datos{
		Total: int(r.Total), Pendientes: int(r.Pendientes),
		PorEstado: conteosExp(r.PorEstado), PorTipo: conteosExp(r.PorTipo), PorPrioridad: conteosExp(r.PorPrioridad),
		PorArea: conteosExp(r.PorArea), PorMes: conteosExp(r.PorMes), Antiguedad: conteosExp(r.PendientesPorAntiguedad),
		Alcance: r.Alcance, AreaAlcance: r.AreaAlcance, Generado: generadoLima(r.Generado),
	}
}
