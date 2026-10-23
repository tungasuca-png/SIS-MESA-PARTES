// Package datos integra el motor universal (tabular) con el maestro de
// Estudiantes: define sus campos, interpreta y clasifica los registros de un
// archivo contra el maestro y arma las filas de exportación.
package datos

import (
	"regexp"

	"tabular"
)

const Modulo = "estudiantes"

// Grupos de campos: los datos del ESTUDIANTE (maestro), los académicos
// HEREDADOS que el maestro aún guarda (columnas nivel/grado/sección/turno de
// "estudiantes", las que muestra hoy la tabla del directorio) y los de su
// MATRÍCULA VIGENTE (matriculas + secciones del periodo EN_CURSO), que solo
// se exportan: la matrícula se registra en su propia pantalla.
const (
	GrupoEstudiante = "Estudiante"
	GrupoAcademico  = "Registro académico (estudiante)"
	GrupoMatricula  = "Matrícula vigente"
)

var (
	patronDNI     = regexp.MustCompile(`^[0-9]{8}$`)
	patronSIAGIE  = regexp.MustCompile(`^[0-9]{14}$`)
	patronCodigo  = regexp.MustCompile(`^[A-Za-z0-9-]{1,20}$`)
	patronDocComb = regexp.MustCompile(`(?i)^\s*(DNI|CE|C\.?E\.?|PASAPORTE|CARNE?T?\s+EXT\.?)\s*[:\-]?\s*[A-Z0-9]{4,12}\s*$`)
)

// Esquema de Estudiantes. Los sinónimos son equivalencias SIN duda en este
// sistema; los dudosos (p. ej. AULA por SECCIÓN, AÑO por GRADO) siempre se
// confirman. Conceptos distintos del modelo no se confunden: NIVEL no es
// GRADO, y ESTADO (del estudiante) no es ESTADO DE MATRÍCULA.
var Esquema = tabular.Esquema{Modulo: Modulo, Campos: []tabular.Campo{
	{Clave: "codigo", Etiqueta: "Código", Grupo: GrupoEstudiante, Importable: true, Exportable: true,
		Sinonimos:        []string{"CODIGO", "CODIGO ESTUDIANTE", "CODIGO DEL ESTUDIANTE", "CODIGO INSTITUCIONAL", "COD ESTUDIANTE", "CODIGO ALUMNO"},
		SinonimosDudosos: []string{"COD", "ID", "IDENTIFICADOR"}},
	{Clave: "codigo_siagie", Etiqueta: "Código SIAGIE", Grupo: GrupoEstudiante, Importable: true, Exportable: true,
		Sinonimos: []string{"SIAGIE", "CODIGO SIAGIE", "COD SIAGIE", "CODIGO DEL ESTUDIANTE SIAGIE"}, Patron: patronSIAGIE},
	{Clave: "tipo_documento", Etiqueta: "Tipo de documento", Grupo: GrupoEstudiante, Importable: true, Exportable: true,
		Sinonimos: []string{"TIPO DOC", "TIPO DOCUMENTO", "TIPO DE DOCUMENTO", "TIPO DOC IDENTIDAD", "TIPO DOCUMENTO IDENTIDAD", "TDOC"},
		Catalogo:  []string{"DNI", "CE", "PASAPORTE", "SIN_DOCUMENTO", "CARNE EXT.", "CARNE DE EXTRANJERIA", "SIN DOCUMENTO"}},
	{Clave: "numero_documento", Etiqueta: "Número de documento", Grupo: GrupoEstudiante, Importable: true, Exportable: true,
		Sinonimos: []string{"DNI", "NRO DOC", "NRO DOCUMENTO", "NUMERO DOCUMENTO", "NUMERO DE DOCUMENTO", "N DOCUMENTO", "NUM DOC",
			"NRO DNI", "NUMERO DNI", "DOCUMENTO IDENTIDAD", "DOCUMENTO DE IDENTIDAD", "NRO DOC IDENTIDAD"},
		Patron: patronDNI},
	{Clave: "documento", Etiqueta: "Documento (tipo y número)", Grupo: GrupoEstudiante, Importable: true, Exportable: true,
		Sinonimos: []string{"DOCUMENTO", "DOC IDENTIDAD COMPLETO"}, Patron: patronDocComb},
	{Clave: "apellido_paterno", Etiqueta: "Apellido paterno", Grupo: GrupoEstudiante, Importable: true, Exportable: true,
		Sinonimos: []string{"APELLIDO PATERNO", "AP PATERNO", "APE PATERNO", "PATERNO", "PRIMER APELLIDO"}},
	{Clave: "apellido_materno", Etiqueta: "Apellido materno", Grupo: GrupoEstudiante, Importable: true, Exportable: true,
		Sinonimos: []string{"APELLIDO MATERNO", "AP MATERNO", "APE MATERNO", "MATERNO", "SEGUNDO APELLIDO"}},
	{Clave: "apellidos", Etiqueta: "Apellidos", Grupo: GrupoEstudiante, Importable: true, Exportable: true,
		Sinonimos: []string{"APELLIDOS", "APELLIDO"}},
	{Clave: "nombres", Etiqueta: "Nombres", Grupo: GrupoEstudiante, Importable: true, Exportable: true,
		Sinonimos: []string{"NOMBRES", "NOMBRE", "PRENOMBRES"}},
	{Clave: "nombre_completo", Etiqueta: "Apellidos y nombres", Grupo: GrupoEstudiante, Importable: true, Exportable: true,
		Sinonimos:        []string{"ESTUDIANTE", "ALUMNO", "APELLIDOS Y NOMBRES", "APELLIDOS NOMBRES", "NOMBRE COMPLETO"},
		SinonimosDudosos: []string{"NOMBRES Y APELLIDOS"}},
	{Clave: "condicion", Etiqueta: "Condición", Grupo: GrupoEstudiante, Importable: true, Exportable: true,
		Sinonimos: []string{"CONDICION", "TIPO DE USUARIO", "TIPO USUARIO"}, Catalogo: []string{"ESTUDIANTE"}},
	{Clave: "estado", Etiqueta: "Estado del estudiante", Grupo: GrupoEstudiante, Importable: true, Exportable: true,
		Sinonimos: []string{"ESTADO", "ESTADO ESTUDIANTE", "ESTADO DEL ESTUDIANTE", "ESTADO REGISTRO"},
		Catalogo:  []string{"ACTIVO", "INACTIVO"}},

	{Clave: "nivel", Etiqueta: "Nivel", Grupo: GrupoAcademico, Importable: true, Exportable: true,
		Sinonimos: []string{"NIVEL", "NIVEL EDUCATIVO", "NIVEL ACADEMICO"}, Catalogo: []string{"INICIAL", "PRIMARIA", "SECUNDARIA"}},
	{Clave: "grado", Etiqueta: "Grado", Grupo: GrupoAcademico, Importable: true, Exportable: true,
		Sinonimos: []string{"GRADO", "GRADO ESCOLAR"}, SinonimosDudosos: []string{"ANO", "GRADO ACADEMICO", "ANO ESCOLAR"}},
	{Clave: "seccion", Etiqueta: "Sección", Grupo: GrupoAcademico, Importable: true, Exportable: true,
		Sinonimos: []string{"SECCION"}, SinonimosDudosos: []string{"AULA", "GRUPO"}},
	{Clave: "turno", Etiqueta: "Turno", Grupo: GrupoAcademico, Importable: true, Exportable: true,
		Sinonimos: []string{"TURNO"}, Catalogo: []string{"MAÑANA", "MANANA", "TARDE", "NOCHE"}},

	{Clave: "matricula_periodo", Etiqueta: "Periodo (matrícula vigente)", Grupo: GrupoMatricula, Exportable: true,
		Sinonimos: []string{"PERIODO", "ANO ACADEMICO", "PERIODO ACADEMICO"}},
	{Clave: "matricula_nivel", Etiqueta: "Nivel (matrícula vigente)", Grupo: GrupoMatricula, Exportable: true},
	{Clave: "matricula_grado", Etiqueta: "Grado (matrícula vigente)", Grupo: GrupoMatricula, Exportable: true},
	{Clave: "matricula_seccion", Etiqueta: "Sección (matrícula vigente)", Grupo: GrupoMatricula, Exportable: true},
	{Clave: "matricula_turno", Etiqueta: "Turno (matrícula vigente)", Grupo: GrupoMatricula, Exportable: true},
	{Clave: "estado_matricula", Etiqueta: "Estado de matrícula", Grupo: GrupoMatricula, Exportable: true,
		Sinonimos: []string{"ESTADO MATRICULA", "ESTADO DE MATRICULA", "SITUACION MATRICULA"},
		Catalogo:  []string{"MATRICULADO", "RETIRADO", "TRASLADADO", "CONCLUIDO"}},
}}

// CampoAcciones: columna vacía opcional "Acciones" para formatos externos que
// la exigen (compatibilidad). No es un dato: siempre se exporta vacía.
const CampoAcciones = "acciones"
