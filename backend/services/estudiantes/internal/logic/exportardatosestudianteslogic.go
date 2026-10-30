package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"estudiantes/estudiantes"
	"estudiantes/internal/datos"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"tabular"
)

type ExportarDatosEstudiantesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExportarDatosEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportarDatosEstudiantesLogic {
	return &ExportarDatosEstudiantesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ExportarDatosEstudiantes exporta TODOS los estudiantes que cumplen los
// filtros del directorio (no solo la página visible), con los campos,
// encabezados, orden y formato elegidos. Exige estudiantes.view.
func (l *ExportarDatosEstudiantesLogic) ExportarDatosEstudiantes(in *estudiantes.DatosExportacionRequest) (*estudiantes.ArchivoDatosResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, "estudiantes.view"); err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.DatosExportacionRequest{}
	}
	filtro, err := filtroBusqueda(in.Q, in.Estado, in.Nivel, in.Grado, in.Seccion, in.Turno, in.Orden)
	if err != nil {
		return nil, err
	}
	pedidas := make([]tabular.ColumnaExport, 0, len(in.Columnas))
	for _, c := range in.Columnas {
		pedidas = append(pedidas, tabular.ColumnaExport{Clave: c.Clave, Encabezado: c.Encabezado})
	}
	if len(pedidas) == 0 {
		pedidas = datos.ColumnasTabla
	}
	cols, err := datos.ColumnasExport(pedidas)
	if err != nil {
		return nil, errorDatos(l.Logger, "exportar datos", err)
	}
	formato := strings.ToLower(strings.TrimSpace(in.Formato))
	if formato == "" {
		formato = tabular.SalidaXLSX
	}
	exp, err := tabular.NuevoExportador(cols, tabular.OpcionesExport{
		Formato: formato, Separador: in.Separador, Codificacion: in.Codificacion,
		ProtegerFormulas: in.ProtegerFormulas, Modulo: datos.Modulo, Generado: validation.HoyLima(),
	})
	if err != nil {
		return nil, errorDatos(l.Logger, "exportar datos", err)
	}
	maxFilas := maxFilasExport(*l.svcCtx)
	n, err := l.svcCtx.EstudianteRepository.ExportarDatos(l.ctx, filtro, maxFilas, func(f datos.FilaMaestro) error {
		valores := make([]string, len(cols))
		for i, c := range cols {
			valores[i] = f.ValorExport(c.Clave)
		}
		return exp.Agregar(valores)
	})
	switch {
	case errors.Is(err, repository.ErrExportDemasiados):
		_, _ = exp.Cerrar()
		return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf("export_demasiados: hay %d estudiantes con esos filtros; el máximo por archivo es %d", n, maxFilas))
	case err != nil:
		_, _ = exp.Cerrar()
		return nil, errorSeguro(l.Logger, "exportar datos", "no se pudo exportar los estudiantes", err)
	case n == 0:
		_, _ = exp.Cerrar()
		return nil, status.Error(codes.FailedPrecondition, "export_sin_resultados: no hay estudiantes con los filtros aplicados")
	}
	contenido, err := exp.Cerrar()
	if err != nil {
		l.Errorf("escribir exportación de datos: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo generar el archivo")
	}
	return &estudiantes.ArchivoDatosResponse{
		Nombre:     fmt.Sprintf("estudiantes_tungasuca_%s.%s", validation.HoyLima().Format("2006-01-02"), formato),
		Mime:       tabular.MimeDe(formato),
		Contenido:  contenido,
		Filas:      int32(exp.Filas()),
		Protegidas: int32(exp.Protegidas()),
	}, nil
}
