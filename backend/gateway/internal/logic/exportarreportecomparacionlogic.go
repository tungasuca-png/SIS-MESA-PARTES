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

type ExportarReporteComparacionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExportarReporteComparacionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportarReporteComparacionLogic {
	return &ExportarReporteComparacionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ExportarReporteComparacion: la MISMA comparación en xlsx o csv.
func (l *ExportarReporteComparacionLogic) ExportarReporteComparacion(req *types.ExportarReporteComparacionRequest) (resp *types.ArchivoReporteResponse, err error) {
	formato, err := formatoReporte(req.Formato)
	if err != nil {
		return nil, err
	}
	c, err := calcularComparacion(l.ctx, l.svcCtx, req.Authorization, filtrosReporte{
		Desde: req.Desde, Hasta: req.Hasta, Estado: req.Estado, Tipo: req.Tipo, Prioridad: req.Prioridad, Area: req.Area,
	}, req.Modo, req.DesdeB, req.HastaB)
	if err != nil {
		return nil, err
	}
	doc := reportes.DocumentoComparacion(c.resultado, c.filtros, c.desdeB, c.hastaB, c.alcance, c.areaAlcance, c.generado)
	return archivoReporte(l.Logger, doc, reportes.NombreArchivoDe("comparacion", c.generado, formato), formato)
}
