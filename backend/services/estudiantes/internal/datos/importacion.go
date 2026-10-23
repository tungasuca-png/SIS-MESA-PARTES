package datos

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"estudiantes/internal/excel"
	"estudiantes/internal/validation"

	"tabular"
)

// Categorías de un registro del archivo (excluyentes; ver Clasificar):
//
//	NUEVO          válido y sin coincidencias: se registrará.
//	EXISTENTE      identificado en el maestro por documento, código o SIAGIE: se omite (no se actualiza).
//	DUPLICADO      repite el documento/código/SIAGIE de una fila anterior del archivo: se omite.
//	POR_CONFIRMAR  sin identificadores y con el mismo nombre que estudiantes registrados:
//	               solo se registra si el usuario confirma que es otra persona.
//	CONFLICTO      sus identificadores apuntan a estudiantes distintos o contradicen al registrado.
//	INVALIDO       tiene errores de datos.
const (
	Nuevo        = "NUEVO"
	Existente    = "EXISTENTE"
	Duplicado    = "DUPLICADO"
	PorConfirmar = "POR_CONFIRMAR"
	Conflicto    = "CONFLICTO"
	Invalido     = "INVALIDO"
)

// Registro normalizado, listo para el maestro.
type Registro struct {
	Codigo          string
	CodigoProvisto  bool
	CodigoSiagie    *string
	TipoDocumento   string
	NumeroDocumento *string
	ApellidoPaterno *string
	ApellidoMaterno *string
	Apellidos       string
	Nombres         string
	Estado          string
	Nivel           string
	Grado           string
	Seccion         string
	Turno           string
}

// Registrado: estudiante del maestro hallado por un identificador.
type Registrado struct {
	ID              string
	Codigo          string
	TipoDocumento   string
	NumeroDocumento string
	CodigoSiagie    string
	Apellidos       string
	Nombres         string
}

func (e Registrado) NombreCompleto() string { return e.Apellidos + ", " + e.Nombres }

// SeccionCatalogo: nivel/grado/sección definidos en alguna sección registrada.
type SeccionCatalogo struct{ Nivel, Grado, Seccion string }

// Consultas al maestro que necesita el análisis (solo lectura).
type Consultas interface {
	Identificadores(ctx context.Context, docs [][2]string, codigos, siagies []string) ([]Registrado, error)
	CoincidenciasNombre(ctx context.Context, nombres []string) (map[string]int, error)
	Secciones(ctx context.Context) ([]SeccionCatalogo, error)
}

// FilaResultado: interpretación y clasificación de un registro.
type FilaResultado struct {
	Fila         int
	Registro     Registro
	Categoria    string
	Errores      []string
	Avisos       []string
	Correcciones []string
	Coincidencia *Registrado
}

// Documento legible ("DNI 01234567" / "Sin documento").
func (r Registro) Documento() string {
	if r.NumeroDocumento == nil {
		return "Sin documento"
	}
	return r.TipoDocumento + " " + *r.NumeroDocumento
}

// Resultado del análisis de un archivo completo.
type Resultado struct {
	Filas       []*FilaResultado
	Bloqueantes []string // problemas del mapeo que impiden continuar
	Avisos      []string // avisos generales
}

// Conteo por categoría.
func (r *Resultado) Conteo() map[string]int {
	out := map[string]int{}
	for _, f := range r.Filas {
		out[f.Categoria]++
	}
	return out
}

var (
	sinSeparadores = regexp.MustCompile(`[\s.\-]`)
	docCombinado   = regexp.MustCompile(`(?i)^\s*([A-Za-zÉé.\s_]+?)\s*[:\-]?\s*([A-Za-z0-9]{4,12})\s*$`)
	soloDigitos    = regexp.MustCompile(`^[0-9]+$`)
)

// Analizar interpreta y clasifica todas las filas con el mapeo dado. No
// escribe nada: la vista previa y la confirmación usan exactamente esto.
func Analizar(ctx context.Context, t *tabular.Tabla, m tabular.Mapeo, huellaArchivo string, q Consultas) (*Resultado, error) {
	idx := map[string]int{}
	for campo, col := range m.Indices() {
		if c, ok := Esquema.Campo(campo); ok && c.Importable {
			idx[campo] = col
		}
	}
	res := &Resultado{}
	_, nom := idx["nombres"]
	_, comp := idx["nombre_completo"]
	_, pat := idx["apellido_paterno"]
	_, ape := idx["apellidos"]
	if !nom && !comp {
		res.Bloqueantes = append(res.Bloqueantes, "falta una columna de Nombres (o «Apellidos y nombres»)")
	}
	if !pat && !ape && !comp {
		res.Bloqueantes = append(res.Bloqueantes, "falta una columna de apellidos (paterno, «Apellidos» o «Apellidos y nombres»)")
	}
	if _, ok := idx["estado"]; !ok {
		res.Avisos = append(res.Avisos, "no hay columna de Estado: los estudiantes nuevos se registrarán como ACTIVO")
	}
	if len(res.Bloqueantes) > 0 {
		return res, nil
	}

	catalogo, err := q.Secciones(ctx)
	if err != nil {
		return nil, err
	}
	for _, f := range t.Filas {
		fr := interpretar(f, idx, huellaArchivo)
		verificarCatalogo(fr, catalogo)
		res.Filas = append(res.Filas, fr)
	}
	if err := clasificar(ctx, res.Filas, q); err != nil {
		return nil, err
	}
	return res, nil
}

func interpretar(f tabular.Fila, idx map[string]int, huella string) *FilaResultado {
	fr := &FilaResultado{Fila: f.Numero}
	valor := func(campo string) string {
		if i, ok := idx[campo]; ok {
			return strings.TrimSpace(f.Valor(i))
		}
		return ""
	}
	_, hayCampo := idx["estado"]
	errorf := func(format string, a ...any) { fr.Errores = append(fr.Errores, fmt.Sprintf(format, a...)) }
	for campo := range idx {
		if strings.HasPrefix(valor(campo), "=") {
			e, _ := Esquema.Campo(campo)
			errorf("%s: no se admiten fórmulas", e.Etiqueta)
		}
	}
	r := &fr.Registro

	// Documento: columnas separadas o una combinada ("DNI: 01234567").
	tipo := excel.NormalizarTipoDocumento(valor("tipo_documento"))
	numero := strings.ToUpper(valor("numero_documento"))
	if comb := valor("documento"); comb != "" && tipo == "" && numero == "" {
		numero = strings.ToUpper(comb)
		if m := docCombinado.FindStringSubmatch(comb); m != nil {
			if t := excel.NormalizarTipoDocumento(m[1]); contiene(excel.TiposDocumento, t) {
				tipo, numero = t, strings.ToUpper(m[2])
			}
		}
	}
	if limpio := sinSeparadores.ReplaceAllString(numero, ""); limpio != numero {
		fr.Correcciones = append(fr.Correcciones, fmt.Sprintf("documento «%s» → «%s» (se quitaron espacios, puntos o guiones)", numero, limpio))
		numero = limpio
	}
	switch {
	case tipo == "" && numero == "":
		tipo = validation.TipoSinDocumento
	case tipo == "" && soloDigitos.MatchString(numero) && len(numero) == 8:
		tipo = validation.TipoDNI
		fr.Correcciones = append(fr.Correcciones, "tipo de documento DNI deducido (8 dígitos)")
	case tipo == "":
		errorf("falta el tipo de documento para «%s»", numero)
	}
	r.TipoDocumento, r.NumeroDocumento = tipo, validation.Optional(numero)
	if tipo != "" {
		if !contiene(excel.TiposDocumento, tipo) {
			errorf("tipo de documento «%s» no admitido: use DNI, CE, PASAPORTE o SIN_DOCUMENTO", tipo)
		} else if err := validation.Documento(tipo, r.NumeroDocumento); err != nil {
			msg := mensaje(err)
			if tipo == validation.TipoDNI && soloDigitos.MatchString(numero) && len(numero) < 8 {
				msg += " (si empieza con 0, escríbalo como texto para no perder los ceros)"
			}
			errorf("documento: %s", msg)
		}
	}

	// Nombres y apellidos.
	paterno, materno := validation.CleanText(valor("apellido_paterno")), validation.CleanText(valor("apellido_materno"))
	apellidos, nombres := validation.CleanText(valor("apellidos")), validation.CleanText(valor("nombres"))
	if completo := validation.CleanText(valor("nombre_completo")); completo != "" && (nombres == "" || (paterno == "" && apellidos == "")) {
		partes := strings.SplitN(completo, ",", 2)
		if len(partes) != 2 {
			errorf("«Apellidos y nombres» debe tener el formato APELLIDOS, NOMBRES (o use columnas separadas)")
		} else {
			if apellidos == "" && paterno == "" {
				apellidos = validation.CleanText(partes[0])
			}
			if nombres == "" {
				nombres = validation.CleanText(partes[1])
			}
			fr.Correcciones = append(fr.Correcciones, "apellidos y nombres separados por la coma")
		}
	}
	switch {
	case paterno != "":
		derivado := strings.TrimSpace(paterno + " " + materno)
		if apellidos != "" && !strings.EqualFold(apellidos, derivado) {
			fr.Avisos = append(fr.Avisos, fmt.Sprintf("«Apellidos» (%s) no coincide con paterno + materno; se usa %s", apellidos, derivado))
		}
		r.ApellidoPaterno, r.ApellidoMaterno, r.Apellidos = &paterno, validation.Optional(materno), derivado
	case materno != "":
		errorf("hay apellido materno sin apellido paterno")
	default:
		r.Apellidos = apellidos
	}
	r.Nombres = nombres
	for _, c := range []struct {
		campo, v string
		max      int
	}{{"Nombres", r.Nombres, validation.MaxNombres}, {"Apellidos", r.Apellidos, validation.MaxApellidos}} {
		if err := validation.RequiredText(c.campo, c.v, c.max); err != nil {
			errorf("%s", mensaje(err))
		}
	}
	for _, c := range []struct {
		campo string
		v     *string
	}{{"Apellido paterno", r.ApellidoPaterno}, {"Apellido materno", r.ApellidoMaterno}} {
		if err := validation.OptionalText(c.campo, c.v, validation.MaxApellido); err != nil {
			errorf("%s", mensaje(err))
		}
	}

	// Condición, estado y datos académicos.
	if _, ok := idx["condicion"]; ok {
		if c := strings.ToUpper(SinTildes(valor("condicion"))); c != excel.Condicion {
			if c == "" {
				errorf("la condición está vacía: solo se importan estudiantes")
			} else {
				errorf("condición «%s»: solo se importan estudiantes", valor("condicion"))
			}
		}
	}
	r.Estado = strings.ToUpper(valor("estado"))
	switch {
	case r.Estado == "" && hayCampo:
		r.Estado = validation.EstadoRegistroActivo
		fr.Correcciones = append(fr.Correcciones, "estado vacío: se registra ACTIVO")
	case r.Estado == "":
		r.Estado = validation.EstadoRegistroActivo
	case !validation.IsValidEstadoRegistro(r.Estado):
		errorf("estado «%s» no admitido: use ACTIVO o INACTIVO", valor("estado"))
	}
	r.Nivel = strings.ToUpper(SinTildes(valor("nivel")))
	if r.Nivel != "" && !contiene(excel.Niveles, r.Nivel) {
		errorf("nivel «%s» no admitido: use INICIAL, PRIMARIA o SECUNDARIA", valor("nivel"))
	}
	r.Grado = strings.ToUpper(validation.CleanText(valor("grado")))
	r.Seccion = strings.ToUpper(validation.CleanText(valor("seccion")))
	r.Turno = strings.ToUpper(validation.CleanText(valor("turno")))
	for _, c := range []struct {
		campo, v string
		max      int
	}{{"Grado", r.Grado, 20}, {"Sección", r.Seccion, 10}, {"Turno", r.Turno, 20}} {
		if err := validation.OptionalText(c.campo, validation.Optional(c.v), c.max); err != nil {
			errorf("%s", mensaje(err))
		}
	}

	// Identificadores del maestro.
	if s := valor("codigo_siagie"); s != "" {
		r.CodigoSiagie = &s
		if err := validation.CodigoSiagie(r.CodigoSiagie); err != nil {
			errorf("%s", mensaje(err))
		}
	}
	if c := valor("codigo"); c != "" {
		r.Codigo, r.CodigoProvisto = c, true
		if err := validation.RequiredText("Código", c, validation.MaxCodigo); err != nil {
			errorf("%s", mensaje(err))
		}
	} else if len(fr.Errores) == 0 {
		r.Codigo = excel.CodigoPara(excel.Registro{TipoDocumento: r.TipoDocumento, NumeroDocumento: r.NumeroDocumento}, huella, f.Numero)
	}
	if len(fr.Errores) > 0 {
		fr.Categoria = Invalido
	}
	return fr
}

// verificarCatalogo: nivel/grado/sección que no existen en ninguna sección
// registrada se AVISAN (las columnas académicas heredadas son texto libre).
func verificarCatalogo(fr *FilaResultado, catalogo []SeccionCatalogo) {
	r := fr.Registro
	if len(catalogo) == 0 || fr.Categoria == Invalido || (r.Grado == "" && r.Seccion == "") {
		return
	}
	for _, s := range catalogo {
		if (r.Nivel == "" || strings.EqualFold(s.Nivel, r.Nivel)) &&
			(r.Grado == "" || strings.EqualFold(s.Grado, r.Grado)) &&
			(r.Seccion == "" || strings.EqualFold(s.Seccion, r.Seccion)) {
			return
		}
	}
	fr.Avisos = append(fr.Avisos, fmt.Sprintf("grado/sección «%s %s %s» no coincide con ninguna sección registrada", r.Nivel, r.Grado, r.Seccion))
}

// clasificar compara contra el propio archivo y contra el maestro.
func clasificar(ctx context.Context, filas []*FilaResultado, q Consultas) error {
	var docs [][2]string
	var codigos, siagies, nombres []string
	primeraPor := map[string]int{}
	for _, f := range filas {
		if f.Categoria == Invalido {
			continue
		}
		r := f.Registro
		claves := []string{}
		if r.NumeroDocumento != nil {
			claves = append(claves, "doc:"+r.TipoDocumento+":"+*r.NumeroDocumento)
			docs = append(docs, [2]string{r.TipoDocumento, *r.NumeroDocumento})
		}
		if r.CodigoProvisto {
			claves = append(claves, "cod:"+r.Codigo)
		}
		if r.CodigoSiagie != nil {
			claves = append(claves, "siagie:"+*r.CodigoSiagie)
			siagies = append(siagies, *r.CodigoSiagie)
		}
		codigos = append(codigos, r.Codigo)
		for _, k := range claves {
			if prev, ok := primeraPor[k]; ok {
				f.Categoria = Duplicado
				f.Avisos = append(f.Avisos, fmt.Sprintf("repite %s de la fila %d del archivo", descripcionClave(k), prev))
				break
			}
		}
		if f.Categoria == Duplicado {
			continue
		}
		for _, k := range claves {
			primeraPor[k] = f.Fila
		}
		if r.NumeroDocumento == nil && !r.CodigoProvisto && r.CodigoSiagie == nil {
			nombres = append(nombres, r.Apellidos+" "+r.Nombres)
		}
	}

	existentes, err := q.Identificadores(ctx, docs, codigos, siagies)
	if err != nil {
		return err
	}
	porDoc, porCodigo, porSiagie := map[string]Registrado{}, map[string]Registrado{}, map[string]Registrado{}
	for _, e := range existentes {
		if e.NumeroDocumento != "" {
			porDoc[e.TipoDocumento+":"+e.NumeroDocumento] = e
		}
		porCodigo[e.Codigo] = e
		if e.CodigoSiagie != "" {
			porSiagie[e.CodigoSiagie] = e
		}
	}
	coincidencias, err := q.CoincidenciasNombre(ctx, nombres)
	if err != nil {
		return err
	}

	for _, f := range filas {
		if f.Categoria != "" {
			continue
		}
		r := f.Registro
		hallados := map[string]Registrado{}
		var motivos []string
		if r.NumeroDocumento != nil {
			if e, ok := porDoc[r.TipoDocumento+":"+*r.NumeroDocumento]; ok {
				hallados[e.ID] = e
				motivos = append(motivos, "documento")
			}
		}
		if e, ok := porCodigo[r.Codigo]; ok && r.CodigoProvisto {
			hallados[e.ID] = e
			motivos = append(motivos, "código")
		}
		if r.CodigoSiagie != nil {
			if e, ok := porSiagie[*r.CodigoSiagie]; ok {
				hallados[e.ID] = e
				motivos = append(motivos, "código SIAGIE")
			}
		}
		switch len(hallados) {
		case 0:
			if e, ok := porCodigo[r.Codigo]; ok {
				// El código DERIVADO ya existe: misma fila de este archivo ya
				// importada (sin documento) o un código usado por otra persona.
				if r.NumeroDocumento == nil && strings.HasPrefix(r.Codigo, "SD") {
					f.Categoria, f.Coincidencia = Existente, &e
					f.Avisos = append(f.Avisos, "esta fila de este mismo archivo ya fue importada")
				} else {
					f.Categoria = Conflicto
					f.Errores = append(f.Errores, fmt.Sprintf("el código %s ya pertenece a otro estudiante (%s)", r.Codigo, e.NombreCompleto()))
				}
				continue
			}
			nombre := r.Apellidos + " " + r.Nombres
			if n := coincidencias[nombre]; n > 0 && r.NumeroDocumento == nil && !r.CodigoProvisto && r.CodigoSiagie == nil {
				f.Categoria = PorConfirmar
				f.Avisos = append(f.Avisos, fmt.Sprintf("sin documento ni código, y hay %d estudiante(s) registrado(s) con el mismo nombre: confirme si es otra persona", n))
				continue
			}
			f.Categoria = Nuevo
		case 1:
			var e Registrado
			for _, x := range hallados {
				e = x
			}
			f.Coincidencia = &e
			// Identificadores del archivo que contradicen al registrado.
			var contradicciones []string
			if r.CodigoProvisto && r.Codigo != e.Codigo {
				contradicciones = append(contradicciones, fmt.Sprintf("el archivo indica el código %s y el registrado es %s", r.Codigo, e.Codigo))
			}
			if r.NumeroDocumento != nil && e.NumeroDocumento != "" && (r.TipoDocumento != e.TipoDocumento || *r.NumeroDocumento != e.NumeroDocumento) {
				contradicciones = append(contradicciones, fmt.Sprintf("el archivo indica %s y el registrado tiene %s %s", r.Documento(), e.TipoDocumento, e.NumeroDocumento))
			}
			if r.CodigoSiagie != nil && e.CodigoSiagie != "" && *r.CodigoSiagie != e.CodigoSiagie {
				contradicciones = append(contradicciones, "el código SIAGIE no coincide con el registrado")
			}
			if len(contradicciones) > 0 {
				f.Categoria = Conflicto
				f.Errores = append(f.Errores, "identificadores contradictorios: "+strings.Join(contradicciones, "; "))
				continue
			}
			f.Categoria = Existente
			f.Avisos = append(f.Avisos, fmt.Sprintf("ya registrado (%s, por %s): no se modifica", e.Codigo, strings.Join(motivos, " y ")))
			if !mismoNombre(r.Apellidos+" "+r.Nombres, e.Apellidos+" "+e.Nombres) {
				f.Avisos = append(f.Avisos, fmt.Sprintf("los nombres difieren del registro (%s): revise", e.NombreCompleto()))
			}
		default:
			f.Categoria = Conflicto
			var cods []string
			for _, x := range hallados {
				cods = append(cods, x.Codigo)
			}
			f.Errores = append(f.Errores, "sus identificadores corresponden a estudiantes distintos ("+strings.Join(cods, ", ")+")")
		}
	}
	return nil
}

func descripcionClave(k string) string {
	switch {
	case strings.HasPrefix(k, "doc:"):
		return "el documento"
	case strings.HasPrefix(k, "cod:"):
		return "el código"
	}
	return "el código SIAGIE"
}

// SinTildes para comparar valores de catálogo.
func SinTildes(s string) string { return tabular.SinTildes(s) }

func mismoNombre(a, b string) bool {
	return tabular.NormalizarEncabezado(a) == tabular.NormalizarEncabezado(b)
}

func contiene(lista []string, v string) bool {
	for _, x := range lista {
		if x == v {
			return true
		}
	}
	return false
}

func mensaje(err error) string {
	if e, ok := err.(*validation.Error); ok {
		return e.Msg
	}
	return err.Error()
}
