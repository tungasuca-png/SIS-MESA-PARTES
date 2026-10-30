package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"estudiantes/estudiantes"
	"estudiantes/internal/authorization"
	"estudiantes/internal/datos"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"tabular"
)

// Motor universal de datos: piezas comunes de los RPC de importación,
// exportación y plantillas.

const (
	defaultDatosMaxFilasExport = 10000
	tipoImportacion            = "IMPORTACION"
	tipoExportacion            = "EXPORTACION"
)

// autorizarImportacion: dar de alta (estudiantes.create) y ver el maestro
// (estudiantes.view), porque el análisis muestra los registrados que coinciden.
func autorizarImportacion(ctx context.Context, sc *svc.ServiceContext) (string, error) {
	userID, err := autorizar(ctx, sc, "estudiantes.create")
	if err != nil {
		return "", err
	}
	if err := authorization.RequirePermission(ctx, sc.AuthClient, userID, "estudiantes.view"); err != nil {
		return "", err
	}
	return userID, nil
}

func limitesDatos(c svc.ServiceContext) tabular.Limites {
	l := tabular.LimitesPorDefecto()
	if c.Config.Datos.MaxBytes > 0 {
		l.MaxBytes = c.Config.Datos.MaxBytes
	}
	if c.Config.Datos.MaxFilas > 0 {
		l.MaxFilas = c.Config.Datos.MaxFilas
	}
	return l
}

func maxFilasExport(c svc.ServiceContext) int {
	if c.Config.Datos.MaxFilasExportacion > 0 {
		return c.Config.Datos.MaxFilasExportacion
	}
	return defaultDatosMaxFilasExport
}

// errorDatos traduce errores del motor (400 con su motivo) y de plantillas;
// el resto, con errorSeguro (mensaje genérico).
func errorDatos(log logx.Logger, op string, err error) error {
	if _, ok := status.FromError(err); ok {
		return err
	}
	if code := tabular.CodigoDe(err); code != "" {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	switch {
	case errors.Is(err, repository.ErrPlantillasSinTabla):
		return status.Error(codes.FailedPrecondition, "plantillas_no_instaladas: falta aplicar la migración 010 de Estudiantes")
	case errors.Is(err, repository.ErrPlantillaNotFound):
		return status.Error(codes.NotFound, "plantilla_not_found: la plantilla no existe")
	case errors.Is(err, repository.ErrPlantillaDuplicada):
		return status.Error(codes.AlreadyExists, "duplicate_plantilla: ya existe una plantilla con ese nombre")
	case errors.Is(err, repository.ErrPlantillaVersion):
		return status.Error(codes.Aborted, "version_conflict: la plantilla fue modificada por otro usuario; vuelva a cargarla")
	}
	return errorSeguro(log, op, "no se pudo procesar los datos", err)
}

// configImportacion: lo que guarda una plantilla de importación. Solo
// asociaciones CONFIRMADAS (encabezado normalizado -> campo).
type configImportacion struct {
	Opciones     *opcionesJSON     `json:"opciones,omitempty"`
	Asociaciones map[string]string `json:"asociaciones"`
}

type opcionesJSON struct {
	Hoja           string `json:"hoja,omitempty"`
	Ruta           string `json:"ruta,omitempty"`
	Separador      string `json:"separador,omitempty"`
	Codificacion   string `json:"codificacion,omitempty"`
	FilaEncabezado int    `json:"fila_encabezado,omitempty"`
}

// configExportacion: campos, encabezados, orden y formato de salida.
type configExportacion struct {
	Columnas         []columnaJSON `json:"columnas"`
	Formato          string        `json:"formato"`
	Separador        string        `json:"separador,omitempty"`
	Codificacion     string        `json:"codificacion,omitempty"`
	ProtegerFormulas bool          `json:"proteger_formulas"`
}

type columnaJSON struct {
	Clave      string `json:"clave"`
	Encabezado string `json:"encabezado"`
}

// lecturaImportacion: archivo leído, mapeo usado y huella de la combinación.
type lecturaImportacion struct {
	tabla           *tabular.Tabla
	mapeo           tabular.Mapeo
	huella          string
	huellaArchivo   string
	plantilla       string
	plantillaAvisos []string
	mapeoConfirmado bool
}

// leerImportacion lee el archivo con las opciones (las de la plantilla si el
// usuario no indicó otras) y resuelve el mapeo: el confirmado por el usuario,
// o la sugerencia del motor con las asociaciones de la plantilla.
func leerImportacion(ctx context.Context, sc *svc.ServiceContext, in *estudiantes.DatosImportacionRequest) (*lecturaImportacion, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "archivo_vacio: envíe un archivo")
	}
	op := tabular.Opciones{Limites: limitesDatos(*sc), PuntuarEncabezado: datos.Esquema.PuntuarEncabezado}
	if o := in.Opciones; o != nil {
		op.Hoja, op.Ruta, op.Separador, op.Codificacion, op.FilaEncabezado = o.Hoja, o.Ruta, o.Separador, o.Codificacion, int(o.FilaEncabezado)
	}
	out := &lecturaImportacion{}
	var asociaciones map[string]string
	if id := strings.TrimSpace(in.PlantillaId); id != "" && !in.MapeoConfirmado {
		if _, err := validarID(id); err != nil {
			return nil, err
		}
		p, err := sc.EstudianteRepository.GetPlantilla(ctx, id, datos.Modulo)
		if err != nil {
			return nil, err
		}
		if p.Tipo != tipoImportacion {
			return nil, status.Error(codes.InvalidArgument, "validation_error: la plantilla no es de importación")
		}
		var cfg configImportacion
		if err := json.Unmarshal(p.Configuracion, &cfg); err != nil {
			return nil, status.Error(codes.FailedPrecondition, "plantilla_invalida: la configuración guardada no es válida")
		}
		asociaciones, out.plantilla = cfg.Asociaciones, p.Nombre
		if o := cfg.Opciones; o != nil && in.Opciones == nil {
			op.Hoja, op.Ruta, op.Separador, op.Codificacion, op.FilaEncabezado = o.Hoja, o.Ruta, o.Separador, o.Codificacion, o.FilaEncabezado
		}
	}
	t, err := tabular.Leer(in.NombreArchivo, in.Contenido, op)
	if err != nil {
		return nil, err
	}
	out.tabla = t
	sum := sha256.Sum256(in.Contenido)
	out.huellaArchivo = hex.EncodeToString(sum[:])

	if in.MapeoConfirmado {
		columnas := map[int]string{}
		for _, a := range in.Mapeo {
			if campo := strings.TrimSpace(a.Campo); campo != "" {
				if _, repetida := columnas[int(a.Columna)]; repetida {
					return nil, status.Error(codes.InvalidArgument, "opcion_invalida: una columna tiene dos campos asignados")
				}
				columnas[int(a.Columna)] = campo
			}
		}
		m, err := datos.Esquema.Aplicar(t, columnas)
		if err != nil {
			return nil, err
		}
		out.mapeo, out.mapeoConfirmado = m, true
	} else {
		out.mapeo = datos.Esquema.Sugerir(t, asociaciones)
		if len(asociaciones) > 0 {
			presentes := map[string]bool{}
			for _, enc := range t.Encabezados {
				presentes[tabular.NormalizarEncabezado(enc)] = true
			}
			var faltan []string
			for enc := range asociaciones {
				if !presentes[enc] {
					faltan = append(faltan, enc)
				}
			}
			sort.Strings(faltan)
			if len(faltan) > 0 {
				out.plantillaAvisos = append(out.plantillaAvisos, "el archivo no tiene columnas de la plantilla («"+strings.Join(faltan, "», «")+"»): la estructura cambió, revise el mapeo")
			}
		}
	}
	out.huella = huellaImportacion(out.huellaArchivo, t.Origen, out.mapeo)
	return out, nil
}

// huellaImportacion: archivo + opciones efectivas + mapeo. La confirmación
// exige la misma: lo que se registra es exactamente lo previsualizado.
func huellaImportacion(huellaArchivo string, o tabular.Origen, m tabular.Mapeo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s|%s|%s|%s|%s|%d", huellaArchivo, o.Formato, o.Hoja, o.Ruta, o.Separador, o.Codificacion, o.FilaEncabezado)
	for _, a := range m.Asignaciones {
		fmt.Fprintf(&b, "|%d=%s", a.Columna, a.Campo)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// respuestaDatos arma la respuesta común de análisis y confirmación.
func respuestaDatos(l *lecturaImportacion, res *datos.Resultado, resultados map[int]string) *estudiantes.DatosImportacionResponse {
	o := l.tabla.Origen
	resp := &estudiantes.DatosImportacionResponse{
		Formato: o.Formato, Extension: o.ExtensionDeclarada, Hojas: o.Hojas, Hoja: o.Hoja, Rutas: o.Rutas, Ruta: o.Ruta,
		Separador: o.Separador, Codificacion: o.Codificacion, FilaEncabezado: int32(o.FilaEncabezado), AvisosArchivo: o.Avisos,
		ProblemasMapeo: l.mapeo.Problemas, RequiereConfirmarMapeo: !l.mapeoConfirmado && l.mapeo.RequiereConfirmacion(),
		Huella: l.huella, PlantillaAplicada: l.plantilla, PlantillaAvisos: l.plantillaAvisos, Resumen: &estudiantes.ResumenDatos{},
		Confirmada: resultados != nil,
	}
	for _, a := range l.mapeo.Asignaciones {
		c, _ := datos.Esquema.Campo(a.Campo)
		resp.Columnas = append(resp.Columnas, &estudiantes.ColumnaAnalizada{
			Columna: int32(a.Columna), Encabezado: a.Encabezado, Campo: a.Campo, Confianza: a.Confianza, Motivo: a.Motivo,
			Alternativas: a.Alternativas, Muestras: a.Muestras, Confirmada: a.Confirmada, Importable: a.Campo == "" || c.Importable,
		})
	}
	if res == nil {
		return resp
	}
	resp.Bloqueantes, resp.Avisos = res.Bloqueantes, res.Avisos
	r := resp.Resumen
	for _, f := range res.Filas {
		r.Total++
		switch f.Categoria {
		case datos.Nuevo:
			r.Nuevos++
		case datos.Existente:
			r.Existentes++
		case datos.Duplicado:
			r.Duplicados++
		case datos.PorConfirmar:
			r.PorConfirmar++
		case datos.Conflicto:
			r.Conflictos++
		case datos.Invalido:
			r.Invalidos++
		}
		if len(f.Avisos) > 0 {
			r.ConAvisos++
		}
		reg := f.Registro
		ra := &estudiantes.RegistroAnalizado{
			Fila: int32(f.Fila), Categoria: f.Categoria, Codigo: reg.Codigo, Apellidos: reg.Apellidos, Nombres: reg.Nombres,
			Estado: reg.Estado, Nivel: reg.Nivel, Grado: reg.Grado, Seccion: reg.Seccion, Turno: reg.Turno,
			Errores: f.Errores, Avisos: f.Avisos, Correcciones: f.Correcciones,
		}
		if reg.TipoDocumento != "" {
			ra.Documento = reg.Documento()
		}
		if f.Coincidencia != nil {
			ra.Coincidencia = f.Coincidencia.Codigo + " — " + f.Coincidencia.NombreCompleto()
		}
		if resultados != nil {
			ra.Resultado = resultados[f.Fila]
			if ra.Resultado == "INSERTADO" {
				r.Insertados++
			} else {
				r.Omitidos++
			}
		}
		resp.Registros = append(resp.Registros, ra)
	}
	return resp
}
