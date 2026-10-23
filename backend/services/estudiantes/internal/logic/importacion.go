package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"estudiantes/estudiantes"
	"estudiantes/internal/config"
	"estudiantes/internal/excel"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Importación Excel de estudiantes (formato oficial). Toda la validación y
// la escritura ocurren aquí, en el servicio dueño del maestro: el Gateway
// solo transporta el archivo. La confirmación vuelve a leer y validar el
// archivo completo; nunca confía en el resultado de la vista previa.
//
// Reglas de identidad (decididas con el colegio):
//   - con documento, la clave es (tipo, número) — índice único del maestro.
//     Un estudiante ya registrado NO se modifica: se informa y se omite;
//   - sin documento, la fila se importa; si el nombre coincide con uno
//     registrado (o con otra fila sin documento del archivo) solo se AVISA.
//     El código derivado (excel.CodigoPara) impide que confirmar dos veces el
//     mismo archivo los duplique.

const (
	resValida    = "VALIDA"
	resError     = "ERROR"
	resDuplicada = "DUPLICADA"
	resYaExiste  = "YA_EXISTE"
	resInsertada = "INSERTADA"
	resOmitida   = "OMITIDA"
	resRechazada = "RECHAZADA"
)

const (
	defaultExcelMaxBytes       = 1 << 20
	defaultMaxFilasImportar    = 2000
	defaultMaxFilasExportar    = 10000
	avisoRegistrado            = "ya está registrado en el maestro: no se modifica"
	avisoNombreRegistrado      = "hay un estudiante registrado con el mismo nombre: verifique que no sea la misma persona"
	avisoFilaSinDocumentoHecha = "esta fila de este mismo archivo ya fue importada"
)

func limitesExcel(c config.Config) (excel.Limites, int) {
	lim := excel.Limites{MaxBytes: c.Excel.MaxBytes, MaxFilas: c.Excel.MaxFilasImportacion}
	if lim.MaxBytes <= 0 {
		lim.MaxBytes = defaultExcelMaxBytes
	}
	if lim.MaxFilas <= 0 {
		lim.MaxFilas = defaultMaxFilasImportar
	}
	maxExport := c.Excel.MaxFilasExportacion
	if maxExport <= 0 {
		maxExport = defaultMaxFilasExportar
	}
	return lim, maxExport
}

type filaAnalizada struct {
	fila      excel.Fila
	reg       excel.Registro
	codigo    string
	resultado string
	errores   []string
	avisos    []string
}

type analisis struct {
	huella string
	filas  []*filaAnalizada
}

// errorArchivo traduce los errores de estructura del archivo a 400.
func errorArchivo(err error) error {
	return status.Error(codes.InvalidArgument, err.Error())
}

// analizarImportacion lee, valida y clasifica cada fila contra el propio
// archivo y contra el maestro. No escribe nada.
func analizarImportacion(ctx context.Context, svcCtx *svc.ServiceContext, in *estudiantes.ImportarEstudiantesRequest) (*analisis, error) {
	if in == nil {
		return nil, errorArchivo(excel.ErrArchivoVacio)
	}
	if nombre := strings.ToLower(strings.TrimSpace(in.NombreArchivo)); nombre != "" && !strings.HasSuffix(nombre, ".xlsx") {
		return nil, status.Error(codes.InvalidArgument, "archivo_invalido: solo se aceptan archivos .xlsx (Excel 2007 o posterior)")
	}
	lim, _ := limitesExcel(svcCtx.Config)
	filas, err := excel.Leer(in.Contenido, lim)
	if err != nil {
		return nil, errorArchivo(err)
	}

	a := &analisis{huella: excel.Huella(in.Contenido)}
	documentos := map[string]int{}   // clave de documento -> primera fila
	sinDocumento := map[string]int{} // nombre (sin documento) -> primera fila
	var claves []repository.DocumentoClave
	var codigos, nombres []string

	for _, f := range filas {
		reg, errs := excel.Validar(f)
		fa := &filaAnalizada{fila: f, reg: reg, errores: errs, resultado: resError}
		a.filas = append(a.filas, fa)
		if len(errs) > 0 {
			continue
		}
		fa.codigo = excel.CodigoPara(reg, a.huella, f.Numero)
		if clave := reg.ClaveDocumento(); clave != "" {
			if prev, ok := documentos[clave]; ok {
				fa.resultado = resDuplicada
				fa.errores = append(fa.errores, fmt.Sprintf("el documento se repite: ya figura en la fila %d del archivo", prev))
				continue
			}
			documentos[clave] = f.Numero
			claves = append(claves, repository.DocumentoClave{Tipo: reg.TipoDocumento, Numero: *reg.NumeroDocumento})
		} else {
			nombre := strings.ToUpper(reg.NombreCompleto())
			if prev, ok := sinDocumento[nombre]; ok {
				fa.avisos = append(fa.avisos, fmt.Sprintf("mismo nombre que la fila %d (sin documento): verifique que no sea la misma persona", prev))
			} else {
				sinDocumento[nombre] = f.Numero
			}
			nombres = append(nombres, reg.NombreCompleto())
		}
		fa.resultado = resValida
		codigos = append(codigos, fa.codigo)
	}

	repo := svcCtx.EstudianteRepository
	registrados, err := repo.DocumentosRegistrados(ctx, claves)
	if err != nil {
		return nil, err
	}
	codigosUsados, err := repo.CodigosRegistrados(ctx, codigos)
	if err != nil {
		return nil, err
	}
	nombresUsados, err := repo.NombresRegistrados(ctx, nombres)
	if err != nil {
		return nil, err
	}
	for _, fa := range a.filas {
		if fa.resultado != resValida {
			continue
		}
		clave := fa.reg.ClaveDocumento()
		switch {
		case clave != "" && registrados[clave]:
			fa.resultado = resYaExiste
			fa.avisos = append(fa.avisos, avisoRegistrado)
		case codigosUsados[fa.codigo] && clave == "":
			fa.resultado = resYaExiste
			fa.avisos = append(fa.avisos, avisoFilaSinDocumentoHecha)
		case codigosUsados[fa.codigo]:
			fa.resultado = resError
			fa.errores = append(fa.errores, "el código "+fa.codigo+" ya pertenece a otro estudiante: regístrelo desde el formulario individual")
		case clave == "" && nombresUsados[fa.reg.NombreCompleto()]:
			fa.avisos = append(fa.avisos, avisoNombreRegistrado)
		}
	}
	return a, nil
}

// maestroDe arma el registro del maestro a partir de una fila válida.
func maestroDe(fa *filaAnalizada) repository.Maestro {
	return repository.Maestro{
		Codigo:          fa.codigo,
		TipoDocumento:   fa.reg.TipoDocumento,
		NumeroDocumento: fa.reg.NumeroDocumento,
		Nombres:         fa.reg.Nombres,
		ApellidoPaterno: fa.reg.ApellidoPaterno,
		ApellidoMaterno: fa.reg.ApellidoMaterno,
		Apellidos:       fa.reg.Apellidos,
		Estado:          fa.reg.Estado,
		Nivel:           fa.reg.Nivel,
	}
}

// confirmarFilas inserta las filas VALIDA con la misma lógica de dominio que
// el alta individual (validarMaestro + Create, con su auditoría CREATE).
// Cada fila es su propia transacción: la importación puede ser parcial y el
// resultado de cada fila se informa. Los índices únicos del maestro deciden
// ante dos confirmaciones simultáneas: la segunda recibe "ya existe".
func confirmarFilas(ctx context.Context, svcCtx *svc.ServiceContext, log logx.Logger, userID string, a *analisis) error {
	for _, fa := range a.filas {
		switch fa.resultado {
		case resYaExiste:
			fa.resultado = resOmitida
			continue
		case resError, resDuplicada:
			fa.resultado = resRechazada
			continue
		}
		m := maestroDe(fa)
		if err := validarMaestro(&m); err != nil {
			fa.resultado = resRechazada
			fa.errores = append(fa.errores, mensajeUsuario(err))
			continue
		}
		_, err := svcCtx.EstudianteRepository.Create(ctx, m, repository.Auditoria{
			UsuarioID: userID, Accion: repository.AccionCreate, Campos: camposModificados(&repository.Maestro{}, &m),
		})
		switch {
		case err == nil:
			fa.resultado = resInsertada
		case errors.Is(err, repository.ErrDocumentoExists):
			fa.resultado = resOmitida
			fa.avisos = append(fa.avisos, "se registró mientras tanto (otra importación o usuario): no se modifica")
		case errors.Is(err, repository.ErrCodigoExists) && fa.reg.ClaveDocumento() == "":
			fa.resultado = resOmitida
			fa.avisos = append(fa.avisos, avisoFilaSinDocumentoHecha)
		case errors.Is(err, repository.ErrCodigoExists):
			fa.resultado = resRechazada
			fa.errores = append(fa.errores, "el código "+fa.codigo+" ya pertenece a otro estudiante")
		case errors.Is(err, repository.ErrDocumentoInvalido), errors.Is(err, repository.ErrDatosInvalidos):
			fa.resultado = resRechazada
			fa.errores = append(fa.errores, "la base de datos no aceptó los datos de la fila")
		default:
			// Error inesperado (BD caída, contexto cancelado): se detiene. Lo
			// ya insertado queda; reintentar es seguro (saldrá como OMITIDA).
			return errorSeguro(log, "importar", "no se pudo completar la importación; vuelva a previsualizar el archivo", err)
		}
	}
	return nil
}

// mensajeUsuario: el texto de un error de validación sin su código técnico.
func mensajeUsuario(err error) string {
	var vErr *validation.Error
	if errors.As(err, &vErr) {
		return vErr.Msg
	}
	return "datos no válidos"
}

func respuestaImportacion(a *analisis, confirmada bool) *estudiantes.ImportarEstudiantesResponse {
	resp := &estudiantes.ImportarEstudiantesResponse{Huella: a.huella, Confirmada: confirmada, Resumen: &estudiantes.ResumenImportacion{}}
	r := resp.Resumen
	for _, fa := range a.filas {
		r.Total++
		switch fa.resultado {
		case resValida:
			r.Validas++
		case resError:
			r.ConErrores++
		case resDuplicada:
			r.Duplicadas++
		case resYaExiste:
			r.YaExisten++
		case resInsertada:
			r.Insertadas++
		case resOmitida:
			r.Omitidas++
		case resRechazada:
			r.Rechazadas++
		}
		if len(fa.avisos) > 0 {
			r.ConAvisos++
		}
		f := fa.fila
		resp.Filas = append(resp.Filas, &estudiantes.FilaImportacion{
			Fila:            int32(f.Numero),
			TipoDocumento:   fa.reg.TipoDocumento,
			NumeroDocumento: strings.TrimSpace(f.Documento),
			ApellidoPaterno: validation.CleanText(f.ApellidoPaterno),
			ApellidoMaterno: validation.CleanText(f.ApellidoMaterno),
			Nombres:         validation.CleanText(f.Nombres),
			Estado:          validation.Upper(f.Estado),
			Nivel:           validation.Upper(f.Nivel),
			Resultado:       fa.resultado,
			Errores:         fa.errores,
			Avisos:          fa.avisos,
		})
	}
	return resp
}
