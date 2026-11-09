// Package reconciliacion: reconciliación diaria D8 entre la fuente de verdad
// (expedientes_db: expediente_movimientos + expediente_outbox) y la
// proyección (derivaciones_db: movimientos_proyeccion).
//
// Usa el rol reconciliador_movimientos (solo lectura + registro append-only de
// ejecuciones y discrepancias). En modo REPUBLICACION solo puede pedir, vía
// reconciliacion_republicar(), que se reencole el evento de un movimiento
// detectado SIN PROYECCIÓN en esa misma ejecución. Nunca modifica movimientos,
// proyección ni historial; lo divergente, lo huérfano y lo FALLIDO quedan
// para revisión humana.
package reconciliacion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const Version = "reconciliador-1.0"

const (
	ModoDeteccion     = "DETECCION"
	ModoRepublicacion = "REPUBLICACION"
)

type Discrepancia struct {
	Tipo         string  `json:"tipo"`
	ExpedienteID string  `json:"expediente_id"`
	Secuencia    *int    `json:"secuencia,omitempty"`
	MovimientoID *string `json:"movimiento_id,omitempty"`
	Detalle      string  `json:"detalle"`
}

type Informe struct {
	Ejecucion              string            `json:"ejecucion"`
	Modo                   string            `json:"modo"`
	Version                string            `json:"version_reconciliador"`
	CorteHasta             time.Time         `json:"corte_hasta"`
	ExpedientesRevisados   int               `json:"expedientes_revisados"`
	MovimientosOrigen      int               `json:"movimientos_origen"`
	MovimientosProyectados int               `json:"movimientos_proyectados"`
	Estado                 string            `json:"estado"`
	Discrepancias          []Discrepancia    `json:"discrepancias"`
	Acciones               map[string]string `json:"acciones_republicacion,omitempty"`
	Huella                 string            `json:"-"`
	Archivo                string            `json:"-"`
}

type Opciones struct {
	Modo               string
	Gracia             time.Duration
	MaxRepublicaciones int
	DirectorioInformes string
}

type movimiento struct {
	ID, Exp                   string
	Seq                       int
	Tipo, Actor, Rol, AO, AD  string
	EA, EN, Motivo, Condicion string
	Ocurrido                  time.Time
	Outbox                    *string
}

func ip(i int) *int       { return &i }
func sp(s string) *string { return &s }

// Ejecutar realiza una ejecución completa y la deja registrada y finalizada
// (SIN_DISCREPANCIAS, CON_DISCREPANCIAS o ERROR).
func Ejecutar(ctx context.Context, exp, deriv *pgxpool.Pool, o Opciones) (*Informe, error) {
	if o.Modo != ModoDeteccion && o.Modo != ModoRepublicacion {
		return nil, fmt.Errorf("modo inválido: %q", o.Modo)
	}
	// Discrepancias vacía (no nil): el informe JSON dice [] y no null.
	inf := &Informe{Modo: o.Modo, Version: Version, Acciones: map[string]string{}, Discrepancias: []Discrepancia{}}
	err := exp.QueryRow(ctx, `INSERT INTO reconciliacion_ejecuciones (modo, corte_hasta, version_reconciliador)
		VALUES ($1, clock_timestamp() - $2::interval, $3) RETURNING id::text, corte_hasta`,
		o.Modo, fmt.Sprintf("%d milliseconds", o.Gracia.Milliseconds()), Version).Scan(&inf.Ejecucion, &inf.CorteHasta)
	if err != nil {
		return nil, fmt.Errorf("registrar ejecución: %w", err)
	}
	err = detectar(ctx, exp, deriv, inf)
	if err == nil && o.Modo == ModoRepublicacion {
		err = republicar(ctx, exp, inf, o.MaxRepublicaciones)
	}
	if err != nil {
		inf.Estado = "ERROR"
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = exp.Exec(c, `UPDATE reconciliacion_ejecuciones SET estado = 'ERROR' WHERE id = $1`, inf.Ejecucion)
		return inf, err
	}
	inf.Estado = "SIN_DISCREPANCIAS"
	if len(inf.Discrepancias) > 0 {
		inf.Estado = "CON_DISCREPANCIAS"
	}
	b, _ := json.MarshalIndent(inf, "", "  ")
	sum := sha256.Sum256(b)
	inf.Huella = hex.EncodeToString(sum[:])
	if o.DirectorioInformes != "" {
		if err := os.MkdirAll(o.DirectorioInformes, 0o750); err == nil {
			inf.Archivo = filepath.Join(o.DirectorioInformes, fmt.Sprintf("reconciliacion_%s_%s.json", time.Now().Format("20060102_150405"), inf.Ejecucion))
			if err := os.WriteFile(inf.Archivo, b, 0o440); err != nil {
				inf.Archivo = ""
			}
		}
	}
	_, err = exp.Exec(ctx, `UPDATE reconciliacion_ejecuciones SET estado = $2, expedientes_revisados = $3, movimientos_origen = $4,
		movimientos_proyectados = $5, discrepancias = $6, huella_informe = $7 WHERE id = $1`,
		inf.Ejecucion, inf.Estado, inf.ExpedientesRevisados, inf.MovimientosOrigen, inf.MovimientosProyectados, len(inf.Discrepancias), inf.Huella)
	return inf, err
}

func detectar(ctx context.Context, exp, deriv *pgxpool.Pool, inf *Informe) error {
	// 1. Origen: instantánea consistente (REPEATABLE READ, solo lectura).
	tx, err := exp.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	rows, err := tx.Query(ctx, `SELECT m.id::text, m.expediente_id::text, m.secuencia, m.tipo, m.actor_id::text, m.actor_rol,
		m.area_origen, m.area_destino, m.estado_anterior, m.estado_nuevo, m.motivo, m.condicion, m.ocurrido_en, o.estado
		FROM expediente_movimientos m LEFT JOIN expediente_outbox o ON o.movimiento_id = m.id
		ORDER BY m.expediente_id, m.secuencia`)
	if err != nil {
		return err
	}
	src := map[string]movimiento{}
	porExp := map[string][]movimiento{}
	for rows.Next() {
		var m movimiento
		if err := rows.Scan(&m.ID, &m.Exp, &m.Seq, &m.Tipo, &m.Actor, &m.Rol, &m.AO, &m.AD, &m.EA, &m.EN, &m.Motivo, &m.Condicion, &m.Ocurrido, &m.Outbox); err != nil {
			rows.Close()
			return err
		}
		src[m.ID] = m
		porExp[m.Exp] = append(porExp[m.Exp], m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	type expInfo struct {
		ult          int
		area, estado string
	}
	exps := map[string]expInfo{}
	rows, err = tx.Query(ctx, `SELECT id::text, ultima_secuencia, area_actual, estado FROM expedientes WHERE ultima_secuencia > 0`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		var e expInfo
		if err := rows.Scan(&id, &e.ult, &e.area, &e.estado); err != nil {
			rows.Close()
			return err
		}
		exps[id] = e
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// 2. Proyección: instantánea de solo lectura.
	ptx, err := deriv.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer ptx.Rollback(context.WithoutCancel(ctx))
	rows, err = ptx.Query(ctx, `SELECT movimiento_id::text, expediente_id::text, secuencia, tipo, actor_id::text, actor_rol, area_origen,
		area_destino, estado_anterior, estado_nuevo, motivo, condicion, ocurrido_en FROM movimientos_proyeccion`)
	if err != nil {
		return err
	}
	proy := map[string]movimiento{}
	proyPorExp := map[string][]int{}
	for rows.Next() {
		var m movimiento
		if err := rows.Scan(&m.ID, &m.Exp, &m.Seq, &m.Tipo, &m.Actor, &m.Rol, &m.AO, &m.AD, &m.EA, &m.EN, &m.Motivo, &m.Condicion, &m.Ocurrido); err != nil {
			rows.Close()
			return err
		}
		proy[m.ID] = m
		proyPorExp[m.Exp] = append(proyPorExp[m.Exp], m.Seq)
	}
	rows.Close()
	_ = ptx.Commit(ctx)

	inf.MovimientosOrigen, inf.MovimientosProyectados = len(src), len(proy)
	expSet := map[string]bool{}
	for e := range exps {
		expSet[e] = true
	}
	for e := range porExp {
		expSet[e] = true
	}
	inf.ExpedientesRevisados = len(expSet)
	add := func(d Discrepancia) { inf.Discrepancias = append(inf.Discrepancias, d) }

	// 3. Origen: secuencia completa, coherencia con el expediente, cadena continua.
	for e := range expSet {
		ms := porExp[e]
		var motivos []string
		for i, m := range ms {
			if m.Seq != i+1 {
				motivos = append(motivos, fmt.Sprintf("se esperaba la secuencia %d y aparece la %d", i+1, m.Seq))
				break
			}
		}
		maxSeq := 0
		if len(ms) > 0 {
			maxSeq = ms[len(ms)-1].Seq
		}
		info, ok := exps[e]
		if ok && info.ult != maxSeq {
			motivos = append(motivos, fmt.Sprintf("ultima_secuencia=%d y la mayor registrada es %d", info.ult, maxSeq))
		}
		if len(motivos) > 0 {
			add(Discrepancia{Tipo: "SECUENCIA_ORIGEN_INCOMPLETA", ExpedienteID: e, Detalle: strings.Join(motivos, "; ")})
		}
		if ok && len(ms) > 0 {
			l := ms[len(ms)-1]
			if l.AD != info.area || l.EN != info.estado {
				add(Discrepancia{Tipo: "EXPEDIENTE_INCOHERENTE", ExpedienteID: e,
					Detalle: fmt.Sprintf("expediente %s/%s; último movimiento (%d) %s/%s", info.area, info.estado, l.Seq, l.AD, l.EN)})
			}
		}
		for i := 1; i < len(ms); i++ {
			if ms[i].AO != ms[i-1].AD || ms[i].EA != ms[i-1].EN {
				add(Discrepancia{Tipo: "CADENA_DISCONTINUA", ExpedienteID: e, Secuencia: ip(ms[i].Seq), MovimientoID: sp(ms[i].ID),
					Detalle: fmt.Sprintf("origen %s/%s distinto del destino anterior %s/%s", ms[i].AO, ms[i].EA, ms[i-1].AD, ms[i-1].EN)})
			}
		}
	}

	// 4. Cada movimiento frente a su proyección y su evento.
	for id, m := range src {
		ms := porExp[m.Exp]
		ultimo := ""
		if len(ms) > 0 && ms[len(ms)-1].ID == id {
			ultimo = " (último movimiento del expediente)"
		}
		if m.Outbox != nil && *m.Outbox == "FALLIDO" {
			add(Discrepancia{Tipo: "EVENTO_FALLIDO", ExpedienteID: m.Exp, Secuencia: ip(m.Seq), MovimientoID: sp(id), Detalle: "evento FALLIDO: requiere revisión" + ultimo})
		}
		if p, ok := proy[id]; ok {
			if campos := camposDistintos(m, p); len(campos) > 0 {
				add(Discrepancia{Tipo: "PROYECCION_DIVERGENTE", ExpedienteID: m.Exp, Secuencia: ip(m.Seq), MovimientoID: sp(id), Detalle: "campos distintos: " + strings.Join(campos, ", ")})
			}
			continue
		}
		if m.Ocurrido.After(inf.CorteHasta) {
			continue // dentro de la ventana de gracia
		}
		switch {
		case m.Outbox == nil:
			add(Discrepancia{Tipo: "MOVIMIENTO_SIN_PROYECCION", ExpedienteID: m.Exp, Secuencia: ip(m.Seq), MovimientoID: sp(id), Detalle: "sin proyección y sin evento de outbox" + ultimo})
		case *m.Outbox == "PUBLICADO":
			add(Discrepancia{Tipo: "PUBLICADO_SIN_PROYECCION", ExpedienteID: m.Exp, Secuencia: ip(m.Seq), MovimientoID: sp(id), Detalle: "evento PUBLICADO sin proyección" + ultimo})
		case *m.Outbox == "PENDIENTE":
			add(Discrepancia{Tipo: "EVENTO_RETRASADO", ExpedienteID: m.Exp, Secuencia: ip(m.Seq), MovimientoID: sp(id), Detalle: "evento PENDIENTE más antiguo que la ventana de gracia" + ultimo})
		}
	}

	// 5. Proyecciones sin origen (se descartan las de movimientos posteriores a la instantánea).
	var desconocidos []string
	for id := range proy {
		if _, ok := src[id]; !ok {
			desconocidos = append(desconocidos, id)
		}
	}
	if len(desconocidos) > 0 {
		existen := map[string]bool{}
		rows, err := exp.Query(ctx, `SELECT id::text FROM expediente_movimientos WHERE id = ANY($1::uuid[])`, desconocidos)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			_ = rows.Scan(&id)
			existen[id] = true
		}
		rows.Close()
		for _, id := range desconocidos {
			if !existen[id] {
				p := proy[id]
				add(Discrepancia{Tipo: "PROYECCION_HUERFANA", ExpedienteID: p.Exp, Secuencia: ip(p.Seq), MovimientoID: sp(id), Detalle: "proyección sin movimiento de origen"})
			}
		}
	}

	// 6. Proyección: secuencia completa por expediente.
	for e, seqs := range proyPorExp {
		sort.Ints(seqs)
		for i, s := range seqs {
			if s != i+1 {
				add(Discrepancia{Tipo: "SECUENCIA_PROYECCION_INCOMPLETA", ExpedienteID: e, Detalle: fmt.Sprintf("falta la secuencia %d (aparece la %d)", i+1, s)})
				break
			}
		}
	}

	ordenar(inf.Discrepancias)
	for _, d := range inf.Discrepancias {
		if _, err := exp.Exec(ctx, `INSERT INTO reconciliacion_discrepancias (ejecucion_id, tipo, expediente_id, secuencia, movimiento_id, detalle)
			VALUES ($1, $2, $3, $4, $5, $6)`, inf.Ejecucion, d.Tipo, d.ExpedienteID, d.Secuencia, d.MovimientoID, truncar(d.Detalle, 500)); err != nil {
			return fmt.Errorf("registrar discrepancia: %w", err)
		}
	}
	return nil
}

func camposDistintos(m, p movimiento) []string {
	var c []string
	pares := []struct {
		n    string
		a, b any
	}{{"expediente_id", m.Exp, p.Exp}, {"secuencia", m.Seq, p.Seq}, {"tipo", m.Tipo, p.Tipo}, {"actor_id", m.Actor, p.Actor},
		{"actor_rol", m.Rol, p.Rol}, {"area_origen", m.AO, p.AO}, {"area_destino", m.AD, p.AD}, {"estado_anterior", m.EA, p.EA},
		{"estado_nuevo", m.EN, p.EN}, {"motivo", m.Motivo, p.Motivo}, {"condicion", m.Condicion, p.Condicion}}
	for _, x := range pares {
		if x.a != x.b {
			c = append(c, x.n)
		}
	}
	if !m.Ocurrido.Equal(p.Ocurrido) {
		c = append(c, "ocurrido_en")
	}
	return c
}

func ordenar(ds []Discrepancia) {
	sort.Slice(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		if a.ExpedienteID != b.ExpedienteID {
			return a.ExpedienteID < b.ExpedienteID
		}
		sa, sb := 0, 0
		if a.Secuencia != nil {
			sa = *a.Secuencia
		}
		if b.Secuencia != nil {
			sb = *b.Secuencia
		}
		if sa != sb {
			return sa < sb
		}
		return a.Tipo < b.Tipo
	})
}

func truncar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func republicar(ctx context.Context, exp *pgxpool.Pool, inf *Informe, max int) error {
	n := 0
	for _, d := range inf.Discrepancias {
		if d.Tipo != "MOVIMIENTO_SIN_PROYECCION" && d.Tipo != "PUBLICADO_SIN_PROYECCION" {
			continue
		}
		if n >= max {
			break
		}
		var r string
		if err := exp.QueryRow(ctx, `SELECT reconciliacion_republicar($1, $2)`, inf.Ejecucion, *d.MovimientoID).Scan(&r); err != nil {
			return fmt.Errorf("republicar %s: %w", *d.MovimientoID, err)
		}
		inf.Acciones[*d.MovimientoID] = r
		n++
	}
	return nil
}
