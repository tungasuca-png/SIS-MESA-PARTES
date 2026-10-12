package repository

import (
	"context"
)

// Modelo maestro, etapa 3 — MATRÍCULA (solo lectura en esta etapa).

type Periodo struct {
	ID     string
	Anio   int
	Nombre string
	Estado string
}

type Seccion struct {
	ID      string
	Nivel   string
	Grado   string
	Seccion string
	Turno   *string
	Aula    *string
}

type Movimiento struct {
	Tipo              string
	Fecha             string
	SeccionAnteriorID *string
	SeccionNuevaID    *string
}

type Matricula struct {
	ID              string
	Periodo         Periodo
	Seccion         *Seccion // nil = sin sección asignada
	EstadoMatricula string
	FechaMatricula  *string
	FechaRetiro     *string
	Origen          string
	Version         int
	Movimientos     []Movimiento
}

// ListMatriculas devuelve el historial académico del estudiante: una
// matrícula por periodo (más reciente primero) con sus movimientos en orden
// cronológico. ErrEstudianteNotFound si el estudiante no existe (una lista
// vacía significa "sin matrículas registradas").
func (r *EstudianteRepository) ListMatriculas(ctx context.Context, estudianteID string) ([]Matricula, error) {
	var existe bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM estudiantes WHERE id = $1)`, estudianteID).Scan(&existe); err != nil {
		return nil, err
	}
	if !existe {
		return nil, ErrEstudianteNotFound
	}

	rows, err := r.db.Query(ctx, `
		SELECT m.id, p.id, p.anio, p.nombre, p.estado,
			s.id, s.nivel, s.grado, s.seccion, s.turno, s.aula,
			m.estado_matricula, to_char(m.fecha_matricula, 'YYYY-MM-DD'), to_char(m.fecha_retiro, 'YYYY-MM-DD'),
			m.origen, m.version
		FROM matriculas m
		JOIN periodos_academicos p ON p.id = m.periodo_id
		LEFT JOIN secciones s ON s.id = m.seccion_id
		WHERE m.estudiante_id = $1
		ORDER BY p.anio DESC, m.id
	`, estudianteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matriculas := make([]Matricula, 0)
	idx := map[string]int{}
	ids := make([]string, 0)
	for rows.Next() {
		var m Matricula
		var sID, sNivel, sGrado, sSeccion *string
		var sTurno, sAula *string
		var anio int16
		if err := rows.Scan(&m.ID, &m.Periodo.ID, &anio, &m.Periodo.Nombre, &m.Periodo.Estado,
			&sID, &sNivel, &sGrado, &sSeccion, &sTurno, &sAula,
			&m.EstadoMatricula, &m.FechaMatricula, &m.FechaRetiro, &m.Origen, &m.Version); err != nil {
			return nil, err
		}
		m.Periodo.Anio = int(anio)
		if sID != nil {
			m.Seccion = &Seccion{ID: *sID, Nivel: *sNivel, Grado: *sGrado, Seccion: *sSeccion, Turno: sTurno, Aula: sAula}
		}
		idx[m.ID] = len(matriculas)
		ids = append(ids, m.ID)
		matriculas = append(matriculas, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return matriculas, nil
	}

	movs, err := r.db.Query(ctx, `
		SELECT matricula_id, tipo, to_char(fecha, 'YYYY-MM-DD'), seccion_anterior_id::text, seccion_nueva_id::text
		FROM matricula_movimientos
		WHERE matricula_id = ANY($1)
		ORDER BY created_at, id
	`, ids)
	if err != nil {
		return nil, err
	}
	defer movs.Close()
	for movs.Next() {
		var matriculaID string
		var mv Movimiento
		if err := movs.Scan(&matriculaID, &mv.Tipo, &mv.Fecha, &mv.SeccionAnteriorID, &mv.SeccionNuevaID); err != nil {
			return nil, err
		}
		i := idx[matriculaID]
		matriculas[i].Movimientos = append(matriculas[i].Movimientos, mv)
	}
	return matriculas, movs.Err()
}
