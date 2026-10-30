package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/datos"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"

	"tabular"
)

type CamposDatosEstudiantesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCamposDatosEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CamposDatosEstudiantesLogic {
	return &CamposDatosEstudiantesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Formatos de entrada realmente implementados y probados en tabular.
var formatosEntrada = []string{"xlsx", "xlsm", "ods", "xls (HTML exportado por sistemas web)", "csv", "tsv", "txt", "json", "jsonl", "ndjson", "xml", "html"}

// CamposDatosEstudiantes: campos del módulo (con su grupo y si se importan o
// exportan) y formatos admitidos, para armar la interfaz.
func (l *CamposDatosEstudiantesLogic) CamposDatosEstudiantes(_ *estudiantes.CamposDatosRequest) (*estudiantes.CamposDatosResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, "estudiantes.view"); err != nil {
		return nil, err
	}
	resp := &estudiantes.CamposDatosResponse{FormatosEntrada: formatosEntrada, FormatosSalida: tabular.SalidasSoportadas}
	for _, c := range datos.Esquema.Campos {
		resp.Campos = append(resp.Campos, &estudiantes.CampoDatos{
			Clave: c.Clave, Etiqueta: c.Etiqueta, Grupo: c.Grupo, Importable: c.Importable, Exportable: c.Exportable,
		})
	}
	resp.Campos = append(resp.Campos, &estudiantes.CampoDatos{Clave: datos.CampoAcciones, Etiqueta: "Acciones (columna vacía de compatibilidad)", Grupo: "Compatibilidad", Exportable: true})
	for _, c := range datos.ColumnasTabla {
		resp.ColumnasTabla = append(resp.ColumnasTabla, &estudiantes.ColumnaExportDatos{Clave: c.Clave, Encabezado: c.Encabezado})
	}
	return resp, nil
}
