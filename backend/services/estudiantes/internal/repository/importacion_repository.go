package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Consultas de apoyo para la importación/exportación Excel (lectura del
// maestro). Las altas usan Create, igual que el formulario individual.

// ErrExportDemasiados: la exportación supera el máximo configurado.
var ErrExportDemasiados = errors.New("export_demasiados: la exportación supera el máximo de registros")

// DocumentoClave identifica un documento (índice uq_estudiantes_documento).
type DocumentoClave struct {
	Tipo   string
	Numero string
}

// DocumentosRegistrados devuelve, de las claves dadas, las que ya existen en
// el maestro ("TIPO:NUMERO").
func (r *EstudianteRepository) DocumentosRegistrados(ctx context.Context, claves []DocumentoClave) (map[string]bool, error) {
	out := map[string]bool{}
	if len(claves) == 0 {
		return out, nil
	}
	tipos := make([]string, len(claves))
	numeros := make([]string, len(claves))
	for i, c := range claves {
		tipos[i], numeros[i] = c.Tipo, c.Numero
	}
	rows, err := r.db.Query(ctx, `
		SELECT e.tipo_documento, e.numero_documento
		FROM estudiantes e
		JOIN unnest($1::text[], $2::text[]) AS c(tipo, numero)
		  ON e.tipo_documento = c.tipo AND e.numero_documento = c.numero
	`, tipos, numeros)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tipo, numero string
		if err := rows.Scan(&tipo, &numero); err != nil {
			return nil, err
		}
		out[tipo+":"+numero] = true
	}
	return out, rows.Err()
}

// CodigosRegistrados devuelve los códigos dados que ya existen.
func (r *EstudianteRepository) CodigosRegistrados(ctx context.Context, codigos []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(codigos) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `SELECT codigo FROM estudiantes WHERE codigo = ANY($1::text[])`, codigos)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var codigo string
		if err := rows.Scan(&codigo); err != nil {
			return nil, err
		}
		out[codigo] = true
	}
	return out, rows.Err()
}

// NombresRegistrados devuelve, de los nombres completos dados ("apellidos
// nombres"), los que coinciden con un estudiante registrado, comparando con
// estudiantes_normalizar (sin tildes, minúsculas, espacios colapsados). Solo
// alimenta AVISOS: nunca decide que dos personas son la misma.
func (r *EstudianteRepository) NombresRegistrados(ctx context.Context, nombres []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(nombres) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		WITH registrados AS (
			SELECT DISTINCT estudiantes_normalizar(apellidos || ' ' || nombres) AS k FROM estudiantes
		)
		SELECT DISTINCT x FROM unnest($1::text[]) AS x
		JOIN registrados ON registrados.k = estudiantes_normalizar(x)
	`, nombres)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var nombre string
		if err := rows.Scan(&nombre); err != nil {
			return nil, err
		}
		out[nombre] = true
	}
	return out, rows.Err()
}

// FiltroExport: vacío = sin filtro. Los valores ya vienen validados contra
// los catálogos por la lógica.
type FiltroExport struct {
	Estado string
	Nivel  string
}

// FilaMaestroExport: solo las columnas del formato oficial.
type FilaMaestroExport struct {
	TipoDocumento   string
	NumeroDocumento string
	ApellidoPaterno string
	ApellidoMaterno string
	Apellidos       string
	Nombres         string
	Estado          string
	Nivel           string
}

// Exportar recorre el maestro (orden: apellidos, nombres, id) llamando a fn
// por cada estudiante, sin cargar todo en memoria. Si el total supera
// limite, no recorre nada y devuelve ErrExportDemasiados.
func (r *EstudianteRepository) Exportar(ctx context.Context, filtro FiltroExport, limite int, fn func(FilaMaestroExport) error) error {
	conds := []string{"TRUE"}
	args := []any{}
	for _, c := range []struct{ col, val string }{{"estado", filtro.Estado}, {"nivel", filtro.Nivel}} {
		if c.val != "" {
			args = append(args, c.val)
			conds = append(conds, fmt.Sprintf("%s = $%d", c.col, len(args)))
		}
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM estudiantes WHERE `+where, args...).Scan(&total); err != nil {
		return err
	}
	if limite > 0 && total > limite {
		return ErrExportDemasiados
	}
	rows, err := r.db.Query(ctx, `
		SELECT tipo_documento, COALESCE(numero_documento, ''), COALESCE(apellido_paterno, ''),
		       COALESCE(apellido_materno, ''), apellidos, nombres, estado, nivel
		FROM estudiantes WHERE `+where+`
		ORDER BY apellidos, nombres, id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var f FilaMaestroExport
		if err := rows.Scan(&f.TipoDocumento, &f.NumeroDocumento, &f.ApellidoPaterno, &f.ApellidoMaterno,
			&f.Apellidos, &f.Nombres, &f.Estado, &f.Nivel); err != nil {
			return err
		}
		if err := fn(f); err != nil {
			return err
		}
	}
	return rows.Err()
}
