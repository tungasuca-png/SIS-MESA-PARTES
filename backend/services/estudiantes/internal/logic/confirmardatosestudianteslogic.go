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

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ConfirmarDatosEstudiantesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfirmarDatosEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmarDatosEstudiantesLogic {
	return &ConfirmarDatosEstudiantesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ConfirmarDatosEstudiantes registra los estudiantes NUEVOS (y los "por
// confirmar" que el usuario aceptó) en UNA transacción. Reglas:
//   - el mapeo debe venir confirmado y la huella ser la de la vista previa;
//   - filas inválidas o en conflicto bloquean, salvo que el usuario confirme
//     excluirlas; cada "por confirmar" debe estar aceptada u omitida;
//   - nunca se actualiza ni reemplaza un estudiante existente;
//   - si algo cambió mientras tanto (otro registró el mismo documento), no
//     se guarda NADA y se pide volver a analizar.
func (l *ConfirmarDatosEstudiantesLogic) ConfirmarDatosEstudiantes(in *estudiantes.DatosImportacionRequest) (*estudiantes.DatosImportacionResponse, error) {
	userID, err := autorizarImportacion(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	if in == nil || !in.MapeoConfirmado || strings.TrimSpace(in.Huella) == "" {
		return nil, status.Error(codes.InvalidArgument, "importacion_sin_vista_previa: revise la vista previa (con el mapeo confirmado) antes de confirmar")
	}
	lectura, err := leerImportacion(l.ctx, l.svcCtx, in)
	if err != nil {
		return nil, errorDatos(l.Logger, "confirmar datos", err)
	}
	if lectura.huella != strings.TrimSpace(in.Huella) {
		return nil, status.Error(codes.FailedPrecondition, "importacion_cambiada: el archivo, las opciones o el mapeo no son los de la vista previa; vuelva a analizarlo")
	}
	res, err := datos.Analizar(l.ctx, lectura.tabla, lectura.mapeo, lectura.huellaArchivo, l.svcCtx.EstudianteRepository)
	if err != nil {
		return nil, errorDatos(l.Logger, "confirmar datos", err)
	}
	if len(res.Bloqueantes) > 0 {
		return nil, status.Error(codes.FailedPrecondition, "mapeo_incompleto: "+strings.Join(res.Bloqueantes, "; "))
	}

	aceptadas, omitidas := conjunto(in.Aceptadas), conjunto(in.Omitidas)
	criticas, sinDecidir := 0, 0
	var lote []repository.Maestro
	var filasLote []int
	for _, f := range res.Filas {
		switch f.Categoria {
		case datos.Invalido, datos.Conflicto:
			criticas++
		case datos.PorConfirmar:
			switch {
			case aceptadas[f.Fila]:
				lote, filasLote = append(lote, maestroDeRegistro(f.Registro)), append(filasLote, f.Fila)
			case !omitidas[f.Fila]:
				sinDecidir++
			}
		case datos.Nuevo:
			lote, filasLote = append(lote, maestroDeRegistro(f.Registro)), append(filasLote, f.Fila)
		}
	}
	if criticas > 0 && !in.ExcluirInvalidos {
		return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf(
			"importacion_con_errores: hay %d fila(s) con errores o conflictos; corríjalas o confirme excluirlas", criticas))
	}
	if sinDecidir > 0 {
		return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf(
			"importacion_sin_decidir: %d posible(s) duplicado(s) sin decidir: indique si se importan o se omiten", sinDecidir))
	}
	for i := range lote {
		if err := validarMaestro(&lote[i]); err != nil {
			return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf("validation_error: fila %d: %s", filasLote[i], mensajeUsuario(err)))
		}
	}

	resultados := map[int]string{}
	if len(lote) > 0 {
		_, err := l.svcCtx.EstudianteRepository.CrearLote(l.ctx, lote,
			repository.Auditoria{UsuarioID: userID, Accion: repository.AccionCreate},
			func(m repository.Maestro) []string { return camposModificados(&repository.Maestro{}, &m) })
		var errLote *repository.ErrLote
		if errors.As(err, &errLote) && (errors.Is(err, repository.ErrDocumentoExists) || errors.Is(err, repository.ErrCodigoExists) || errors.Is(err, repository.ErrCodigoSiagieExists)) {
			return nil, status.Error(codes.Aborted, fmt.Sprintf(
				"importacion_conflicto: la fila %d ya fue registrada por otro usuario mientras tanto; no se guardó nada, vuelva a analizar el archivo", filasLote[errLote.Indice]))
		}
		if err != nil {
			return nil, errorSeguro(l.Logger, "confirmar datos", "no se pudo completar la importación; no se guardó ningún estudiante", err)
		}
	}
	for _, fila := range filasLote {
		resultados[fila] = "INSERTADO"
	}
	for _, f := range res.Filas {
		if _, ok := resultados[f.Fila]; !ok {
			resultados[f.Fila] = "OMITIDO"
		}
	}
	resp := respuestaDatos(lectura, res, resultados)
	// Registro de la operación, sin datos personales.
	l.Infof("importación de datos de estudiantes: usuario=%s formato=%s huella=%s registros=%d insertados=%d omitidos=%d",
		userID, lectura.tabla.Origen.Formato, lectura.huellaArchivo, resp.Resumen.Total, resp.Resumen.Insertados, resp.Resumen.Omitidos)
	return resp, nil
}

func conjunto(filas []int32) map[int]bool {
	out := map[int]bool{}
	for _, f := range filas {
		out[int(f)] = true
	}
	return out
}

func maestroDeRegistro(r datos.Registro) repository.Maestro {
	return repository.Maestro{
		Codigo: r.Codigo, CodigoSiagie: r.CodigoSiagie, TipoDocumento: r.TipoDocumento, NumeroDocumento: r.NumeroDocumento,
		Nombres: r.Nombres, ApellidoPaterno: r.ApellidoPaterno, ApellidoMaterno: r.ApellidoMaterno, Apellidos: r.Apellidos,
		Estado: r.Estado, Nivel: r.Nivel, Grado: r.Grado, Seccion: r.Seccion, Turno: r.Turno,
	}
}
