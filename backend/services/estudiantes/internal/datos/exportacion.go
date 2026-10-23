package datos

import (
	"strconv"
	"strings"

	"tabular"
)

// FilaMaestro: un estudiante con su matrícula vigente (si la tiene), tal
// como lo entrega el repositorio para exportar. Una fila por estudiante.
type FilaMaestro struct {
	Codigo, CodigoSiagie                     string
	TipoDocumento, NumeroDocumento           string
	ApellidoPaterno, ApellidoMaterno         string
	Apellidos, Nombres, Estado               string
	Nivel, Grado, Seccion, Turno             string
	MatPeriodo                               int
	MatNivel, MatGrado, MatSeccion, MatTurno string
	EstadoMatricula                          string
}

// ValorExport devuelve el valor de un campo exportable. Sin matrícula
// vigente, los campos de matrícula quedan vacíos (no se inventan).
func (f FilaMaestro) ValorExport(campo string) string {
	switch campo {
	case "codigo":
		return f.Codigo
	case "codigo_siagie":
		return f.CodigoSiagie
	case "tipo_documento":
		return f.TipoDocumento
	case "numero_documento":
		return f.NumeroDocumento
	case "documento":
		if f.NumeroDocumento == "" {
			return "Sin documento"
		}
		return f.TipoDocumento + ": " + f.NumeroDocumento
	case "apellido_paterno":
		return f.ApellidoPaterno
	case "apellido_materno":
		return f.ApellidoMaterno
	case "apellidos":
		return f.Apellidos
	case "nombres":
		return f.Nombres
	case "nombre_completo":
		// Mismo formato que la tabla del directorio: "APELLIDOS, NOMBRES".
		return strings.TrimSuffix(strings.TrimSpace(f.Apellidos+", "+f.Nombres), ",")
	case "condicion":
		return "ESTUDIANTE"
	case "estado":
		return f.Estado
	case "nivel":
		return f.Nivel
	case "grado":
		return f.Grado
	case "seccion":
		return f.Seccion
	case "turno":
		return f.Turno
	case "matricula_periodo":
		if f.MatPeriodo == 0 {
			return ""
		}
		return strconv.Itoa(f.MatPeriodo)
	case "matricula_nivel":
		return f.MatNivel
	case "matricula_grado":
		return f.MatGrado
	case "matricula_seccion":
		return f.MatSeccion
	case "matricula_turno":
		return f.MatTurno
	case "estado_matricula":
		return f.EstadoMatricula
	}
	return "" // "acciones" (compatibilidad) y cualquier otro: vacío
}

// ColumnasExport valida los campos pedidos (exportables, sin repetir) y
// completa los encabezados vacíos con la etiqueta del campo.
func ColumnasExport(pedidas []tabular.ColumnaExport) ([]tabular.ColumnaExport, error) {
	if len(pedidas) == 0 {
		return nil, &tabular.Error{Code: tabular.ErrOpcion, Msg: "elija al menos un campo para exportar"}
	}
	vistos := map[string]bool{}
	out := make([]tabular.ColumnaExport, 0, len(pedidas))
	for _, p := range pedidas {
		clave := strings.TrimSpace(p.Clave)
		enc := strings.TrimSpace(p.Encabezado)
		if clave == CampoAcciones {
			if enc == "" {
				enc = "Acciones"
			}
		} else {
			c, ok := Esquema.Campo(clave)
			if !ok || !c.Exportable {
				return nil, &tabular.Error{Code: tabular.ErrOpcion, Msg: "el campo «" + clave + "» no existe o no se puede exportar"}
			}
			if enc == "" {
				enc = c.Etiqueta
			}
		}
		if vistos[clave] {
			return nil, &tabular.Error{Code: tabular.ErrOpcion, Msg: "el campo «" + clave + "» está repetido"}
		}
		if len([]rune(enc)) > 80 {
			return nil, &tabular.Error{Code: tabular.ErrOpcion, Msg: "el encabezado de «" + clave + "» es demasiado largo"}
		}
		vistos[clave] = true
		out = append(out, tabular.ColumnaExport{Clave: clave, Encabezado: enc})
	}
	return out, nil
}

// ColumnasTabla: las columnas de la tabla del directorio (sin «Acciones»),
// la configuración inicial de la exportación.
var ColumnasTabla = []tabular.ColumnaExport{
	{Clave: "codigo", Encabezado: "Código"},
	{Clave: "nombre_completo", Encabezado: "Estudiante"},
	{Clave: "documento", Encabezado: "Documento"},
	{Clave: "grado", Encabezado: "Grado"},
	{Clave: "seccion", Encabezado: "Sección"},
	{Clave: "estado", Encabezado: "Estado"},
}
