package logic

import (
	"context"
	"errors"
	"fmt"

	"estudiantes/estudiantes"
	"estudiantes/internal/excel"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ExportarEstudiantesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExportarEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportarEstudiantesLogic {
	return &ExportarEstudiantesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ExportarEstudiantes genera el Excel oficial con los datos ACTUALES del
// maestro. Exige estudiantes.view, el mismo permiso que ya da acceso al
// directorio (con su número de documento). No incluye identificadores
// internos ni datos personales, familiares, de matrícula o psicológicos.
func (l *ExportarEstudiantesLogic) ExportarEstudiantes(in *estudiantes.ExportarEstudiantesRequest) (*estudiantes.ArchivoExcelResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, "estudiantes.view"); err != nil {
		return nil, err
	}
	filtro := repository.FiltroExport{}
	if in != nil {
		filtro.Estado, filtro.Nivel = validation.Upper(in.Estado), validation.Upper(in.Nivel)
	}
	if filtro.Estado != "" && !validation.IsValidEstadoRegistro(filtro.Estado) {
		return nil, status.Error(codes.InvalidArgument, "invalid_estado: use ACTIVO o INACTIVO")
	}
	if filtro.Nivel != "" && !contieneValor(excel.Niveles, filtro.Nivel) {
		return nil, status.Error(codes.InvalidArgument, "invalid_nivel: use INICIAL, PRIMARIA o SECUNDARIA")
	}
	_, maxFilas := limitesExcel(l.svcCtx.Config)

	exp, err := excel.NuevoExportador()
	if err != nil {
		l.Errorf("crear exportador de estudiantes: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo generar el archivo")
	}
	err = l.svcCtx.EstudianteRepository.Exportar(l.ctx, filtro, maxFilas, func(r repository.FilaMaestroExport) error {
		paterno := r.ApellidoPaterno
		if paterno == "" {
			// Registro anterior a los apellidos separados: el apellido completo
			// va en la columna del paterno, sin inventar una división.
			paterno = r.Apellidos
		}
		return exp.Agregar(excel.FilaExport{
			TipoDocumento: r.TipoDocumento, Documento: r.NumeroDocumento, ApellidoPaterno: paterno,
			ApellidoMaterno: r.ApellidoMaterno, Nombres: r.Nombres, Estado: r.Estado, Nivel: r.Nivel,
		})
	})
	if err != nil {
		_, _ = exp.Bytes() // libera el archivo temporal del StreamWriter
		if errors.Is(err, repository.ErrExportDemasiados) {
			return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf(
				"export_demasiados: hay más de %d estudiantes; filtre por estado o nivel", maxFilas))
		}
		return nil, errorSeguro(l.Logger, "exportar", "no se pudo exportar los estudiantes", err)
	}
	filas := exp.Filas()
	contenido, err := exp.Bytes()
	if err != nil {
		l.Errorf("escribir exportación de estudiantes: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo generar el archivo")
	}
	nombre := fmt.Sprintf("estudiantes_%s.xlsx", validation.HoyLima().Format("2006-01-02"))
	return &estudiantes.ArchivoExcelResponse{Nombre: nombre, Contenido: contenido, Filas: int32(filas)}, nil
}

func contieneValor(lista []string, v string) bool {
	for _, x := range lista {
		if x == v {
			return true
		}
	}
	return false
}
