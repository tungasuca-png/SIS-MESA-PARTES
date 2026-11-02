// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"gateway/internal/reportes"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExportarReporteActividadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExportarReporteActividadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportarReporteActividadLogic {
	return &ExportarReporteActividadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ExportarReporteActividad: la MISMA actividad por rol en xlsx o csv.
func (l *ExportarReporteActividadLogic) ExportarReporteActividad(req *types.ExportarReportePeriodoRequest) (resp *types.ArchivoReporteResponse, err error) {
	formato, err := formatoReporte(req.Formato)
	if err != nil {
		return nil, err
	}
	a, err := calcularActividad(l.ctx, l.svcCtx, req.Authorization, req.Desde, req.Hasta, req.Tipo)
	if err != nil {
		return nil, err
	}
	doc := reportes.DocumentoActividad(a.roles, filtrosPeriodo(req.Desde, req.Hasta, req.Tipo, ""), a.alcance, a.areaAlcance, a.generado)
	return archivoReporte(l.Logger, doc, reportes.NombreArchivoDe("actividad", a.generado, formato), formato)
}
