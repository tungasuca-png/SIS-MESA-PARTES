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

type ExportarReporteDocumentosLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExportarReporteDocumentosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportarReporteDocumentosLogic {
	return &ExportarReporteDocumentosLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ExportarReporteDocumentos: el MISMO reporte (mismos filtros, permiso y
// alcance) en xlsx o csv.
func (l *ExportarReporteDocumentosLogic) ExportarReporteDocumentos(req *types.ExportarReportePeriodoRequest) (resp *types.ArchivoReporteResponse, err error) {
	formato, err := formatoReporte(req.Formato)
	if err != nil {
		return nil, err
	}
	r, err := consultarDocumentos(l.ctx, l.svcCtx, req.Authorization, req.Desde, req.Hasta, req.Tipo, req.Granularidad, false)
	if err != nil {
		return nil, err
	}
	generado := generadoLima(r.Generado)
	doc := reportes.DocumentoDocumentos(reportes.DatosDocumentos{
		Total: int(r.Total), PorTipoDocumento: conteosDoc(r.PorTipoDocumento), PorTipoTramite: conteosDoc(r.PorTipoTramite),
		Serie: conteosDoc(r.Serie), Granularidad: r.Granularidad, Alcance: r.Alcance, AreaAlcance: r.AreaAlcance, Generado: generado,
	}, filtrosPeriodo(req.Desde, req.Hasta, req.Tipo, r.Granularidad))
	return archivoReporte(l.Logger, doc, reportes.NombreArchivoDe("documentos", generado, formato), formato)
}
