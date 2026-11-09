package reconciliacion

// Prueba de integración D8 con el código real (repositorio de movimientos +
// reconciliador) sobre una base PostgreSQL DESECHABLE instalada desde las
// migraciones del repositorio. Sin las variables, se omite.
//
//	TEST_REC_EXPEDIENTES_USER_DSN     expedientes_user   (crea expedientes y movimientos)
//	TEST_REC_PUBLICADOR_DSN           expedientes_publicador (estados del outbox)
//	TEST_REC_PROYECTOR_DSN            derivaciones_proyector (proyección)
//	TEST_REC_RECONCILIADOR_EXP_DSN    reconciliador_movimientos @ expedientes_db
//	TEST_REC_RECONCILIADOR_DERIV_DSN  reconciliador_movimientos @ derivaciones_db
//
// Las aserciones se limitan a los expedientes creados aquí: la base puede
// contener datos de otras pruebas.

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"expedientes/internal/repository"
)

func entorno(t *testing.T, v string) string {
	t.Helper()
	s := os.Getenv(v)
	if s == "" {
		t.Skipf("%s no definido: prueba de integración D8 omitida", v)
	}
	return s
}

func pool(t *testing.T, ctx context.Context, dsn string) *pgxpool.Pool {
	t.Helper()
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func uuid4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type caso struct {
	exp  *repository.Expediente
	movs []*repository.Movimiento
}

func TestReconciliacionD8Integracion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	user := pool(t, ctx, entorno(t, "TEST_REC_EXPEDIENTES_USER_DSN"))
	pub := pool(t, ctx, entorno(t, "TEST_REC_PUBLICADOR_DSN"))
	proy := pool(t, ctx, entorno(t, "TEST_REC_PROYECTOR_DSN"))
	recExp := pool(t, ctx, entorno(t, "TEST_REC_RECONCILIADOR_EXP_DSN"))
	recDer := pool(t, ctx, entorno(t, "TEST_REC_RECONCILIADOR_DERIV_DSN"))
	repo := repository.NewExpedienteRepository(user)
	actor := uuid4()

	// Crea un expediente con n movimientos reales (DERIVACION SECRETARIA->DIRECTOR,
	// luego CAMBIO_ESTADO EN_PROCESO->ATENDIDO en DIRECTOR).
	crear := func(n int) caso {
		e, err := repo.Create(ctx, &repository.Expediente{Tipo: "SOLICITUD", Asunto: "D8 integración", SolicitanteID: uuid4(), Prioridad: "NORMAL"})
		if err != nil {
			t.Fatal(err)
		}
		c := caso{exp: e}
		pasos := []struct{ tipo, area, estado, rol string }{
			{"DERIVACION", "DIRECTOR", "EN_PROCESO", "SECRETARIA"},
			{"CAMBIO_ESTADO", "DIRECTOR", "ATENDIDO", "DIRECTOR"},
		}
		for _, p := range pasos[:n] {
			p := p
			r, err := repo.RegistrarMovimiento(ctx, repository.SolicitudMovimiento{
				IDOrCodigo: e.ID, Tipo: p.tipo, ActorID: actor, ActorRol: p.rol, IdempotencyKey: uuid4(),
				Contenido: p.area + "|" + p.estado, Motivo: "prueba D8",
				Decidir: func(*repository.Expediente) (repository.Decision, error) {
					return repository.Decision{AreaDestino: p.area, EstadoNuevo: p.estado}, nil
				},
			})
			if err != nil {
				t.Fatalf("movimiento %s: %v", p.tipo, err)
			}
			c.movs = append(c.movs, r.Movimiento)
		}
		return c
	}
	proyectar := func(m *repository.Movimiento, motivo string) {
		if _, err := proy.Exec(ctx, `INSERT INTO movimientos_proyeccion (movimiento_id, expediente_id, secuencia, tipo, actor_id, actor_rol,
			area_origen, area_destino, estado_anterior, estado_nuevo, motivo, condicion, ocurrido_en)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, m.ID, m.ExpedienteID, m.Secuencia, m.Tipo, m.ActorID, m.ActorRol,
			m.AreaOrigen, m.AreaDestino, m.EstadoAnterior, m.EstadoNuevo, motivo, m.Condicion, m.OcurridoEn); err != nil {
			t.Fatalf("proyectar: %v", err)
		}
	}
	outbox := func(m *repository.Movimiento, estado string) {
		// Como el publicador: reclamar (fencing) y cerrar con ese reclamo.
		reclamo := uuid4()
		if _, err := pub.Exec(ctx, `UPDATE expediente_outbox SET reclamo_id = $2, intentos = intentos + 1,
			reclamado_hasta = clock_timestamp() + interval '1 minute' WHERE movimiento_id = $1`, m.ID, reclamo); err != nil {
			t.Fatalf("reclamar: %v", err)
		}
		tag, err := pub.Exec(ctx, `UPDATE expediente_outbox SET estado = $3, reclamo_id = NULL, reclamado_hasta = NULL
			WHERE movimiento_id = $1 AND reclamo_id = $2 AND estado = 'PENDIENTE'`, m.ID, reclamo, estado)
		if err != nil || tag.RowsAffected() != 1 {
			t.Fatalf("outbox %s: %v", estado, err)
		}
	}

	ok := crear(2) // correcto: proyectado y publicado
	for _, m := range ok.movs {
		proyectar(m, m.Motivo)
		outbox(m, "PUBLICADO")
	}
	ultimoPerdido := crear(2) // el ÚLTIMO evento se marcó publicado pero no llegó
	proyectar(ultimoPerdido.movs[0], ultimoPerdido.movs[0].Motivo)
	outbox(ultimoPerdido.movs[0], "PUBLICADO")
	outbox(ultimoPerdido.movs[1], "PUBLICADO")
	retrasado := crear(1) // evento PENDIENTE (publicador caído)
	fallido := crear(1)   // evento FALLIDO
	outbox(fallido.movs[0], "FALLIDO")
	divergente := crear(1) // proyección con otro motivo
	proyectar(divergente.movs[0], "motivo alterado")
	outbox(divergente.movs[0], "PUBLICADO")
	purgado := crear(1) // evento publicado y purgado, sin proyección
	outbox(purgado.movs[0], "PUBLICADO")
	if _, err := pub.Exec(ctx, `DELETE FROM expediente_outbox WHERE movimiento_id = $1`, purgado.movs[0].ID); err != nil {
		t.Fatal(err)
	}
	huerfana := &repository.Movimiento{ID: uuid4(), ExpedienteID: uuid4(), Secuencia: 1, Tipo: "CAMBIO_ESTADO", ActorID: actor,
		ActorRol: "DIRECTOR", AreaOrigen: "DIRECTOR", AreaDestino: "DIRECTOR", EstadoAnterior: "PENDIENTE", EstadoNuevo: "EN_PROCESO",
		OcurridoEn: time.Now().Add(-time.Hour)}
	proyectar(huerfana, "")

	time.Sleep(20 * time.Millisecond)
	nuestros := map[string]bool{}
	for _, c := range []caso{ok, ultimoPerdido, retrasado, fallido, divergente, purgado} {
		nuestros[c.exp.ID] = true
	}
	nuestros[huerfana.ExpedienteID] = true
	filtrar := func(inf *Informe) map[string]string {
		r := map[string]string{}
		for _, d := range inf.Discrepancias {
			if nuestros[d.ExpedienteID] {
				r[d.ExpedienteID] += d.Tipo + " "
			}
		}
		return r
	}

	// 1. DETECCIÓN (gracia mínima: todo lo creado ya quedó fuera de la ventana).
	inf, err := Ejecutar(ctx, recExp, recDer, Opciones{Modo: ModoDeteccion, Gracia: time.Millisecond, DirectorioInformes: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	got := filtrar(inf)
	esperado := map[string]string{
		ultimoPerdido.exp.ID:  "PUBLICADO_SIN_PROYECCION ",
		retrasado.exp.ID:      "EVENTO_RETRASADO ",
		fallido.exp.ID:        "EVENTO_FALLIDO ",
		divergente.exp.ID:     "PROYECCION_DIVERGENTE ",
		purgado.exp.ID:        "MOVIMIENTO_SIN_PROYECCION ",
		huerfana.ExpedienteID: "PROYECCION_HUERFANA ",
	}
	for k, v := range esperado {
		if got[k] != v {
			t.Errorf("expediente %s: obtenido %q, esperado %q", k, got[k], v)
		}
	}
	if _, hay := got[ok.exp.ID]; hay {
		t.Errorf("el expediente correcto no debe tener discrepancias: %q", got[ok.exp.ID])
	}
	if inf.Estado != "CON_DISCREPANCIAS" || len(inf.Huella) != 64 || inf.Archivo == "" {
		t.Fatalf("informe: estado=%s huella=%q archivo=%q", inf.Estado, inf.Huella, inf.Archivo)
	}
	if b, err := os.ReadFile(inf.Archivo); err != nil || !strings.Contains(string(b), inf.Ejecucion) {
		t.Fatalf("informe en disco: %v", err)
	}
	var estado, huella string
	var n int
	if err := recExp.QueryRow(ctx, `SELECT estado, huella_informe, discrepancias FROM reconciliacion_ejecuciones WHERE id = $1`, inf.Ejecucion).
		Scan(&estado, &huella, &n); err != nil || estado != "CON_DISCREPANCIAS" || huella != inf.Huella || n != len(inf.Discrepancias) {
		t.Fatalf("ejecución registrada: %s %s %d %v", estado, huella, n, err)
	}
	// Detalle del último movimiento perdido.
	for _, d := range inf.Discrepancias {
		if d.ExpedienteID == ultimoPerdido.exp.ID && !strings.Contains(d.Detalle, "último movimiento") {
			t.Errorf("debe señalar que es el último movimiento: %q", d.Detalle)
		}
	}

	// 2. Límites de privilegio del reconciliador.
	for _, q := range []string{
		`UPDATE expediente_outbox SET estado = 'PENDIENTE' WHERE movimiento_id = '` + retrasado.movs[0].ID + `'`,
		`INSERT INTO expediente_outbox (movimiento_id, expediente_id, secuencia) VALUES ('` + purgado.movs[0].ID + `','` + purgado.exp.ID + `',1)`,
		`UPDATE expedientes SET area_actual = 'SECRETARIA' WHERE id = '` + ok.exp.ID + `'`,
		`DELETE FROM reconciliacion_discrepancias WHERE ejecucion_id = '` + inf.Ejecucion + `'`,
		`UPDATE reconciliacion_ejecuciones SET estado = 'SIN_DISCREPANCIAS', discrepancias = 0 WHERE id = '` + inf.Ejecucion + `'`,
		`SELECT asunto FROM expedientes LIMIT 1`,
	} {
		if _, err := recExp.Exec(ctx, q); err == nil {
			t.Errorf("el reconciliador NO debía poder: %s", q)
		}
	}
	if _, err := recDer.Exec(ctx, `DELETE FROM movimientos_proyeccion WHERE movimiento_id = $1`, huerfana.ID); err == nil {
		t.Error("el reconciliador NO debía poder borrar proyecciones")
	}
	// Republicar en una ejecución de DETECCIÓN: rechazado.
	if _, err := recExp.Exec(ctx, `SELECT reconciliacion_republicar($1, $2)`, inf.Ejecucion, purgado.movs[0].ID); err == nil {
		t.Error("no debe republicar desde una ejecución de detección")
	}

	// 3. REPUBLICACIÓN controlada (max alto para cubrir los de otras pruebas).
	rep, err := Ejecutar(ctx, recExp, recDer, Opciones{Modo: ModoRepublicacion, Gracia: time.Millisecond, MaxRepublicaciones: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if a := rep.Acciones[ultimoPerdido.movs[1].ID]; a != "REENCOLADO" {
		t.Errorf("último perdido: %q", a)
	}
	if a := rep.Acciones[purgado.movs[0].ID]; a != "EVENTO_RECREADO" {
		t.Errorf("purgado: %q", a)
	}
	for _, m := range []*repository.Movimiento{retrasado.movs[0], fallido.movs[0], divergente.movs[0]} {
		if a, hay := rep.Acciones[m.ID]; hay {
			t.Errorf("no debía actuar sobre %s: %s", m.ID, a)
		}
	}
	// Idempotencia en la misma ejecución: ya finalizada -> rechazado;
	// y el outbox quedó PENDIENTE (lo retoma el publicador).
	if _, err := recExp.Exec(ctx, `SELECT reconciliacion_republicar($1, $2)`, rep.Ejecucion, purgado.movs[0].ID); err == nil {
		t.Error("una ejecución finalizada no admite más republicaciones")
	}
	for _, m := range []*repository.Movimiento{ultimoPerdido.movs[1], purgado.movs[0]} {
		var e string
		if err := recExp.QueryRow(ctx, `SELECT estado FROM expediente_outbox WHERE movimiento_id = $1`, m.ID).Scan(&e); err != nil || e != "PENDIENTE" {
			t.Errorf("outbox de %s: %q %v", m.ID, e, err)
		}
	}
	// Los movimientos de origen no cambiaron.
	var cuenta int
	if err := recExp.QueryRow(ctx, `SELECT count(*) FROM expediente_movimientos WHERE expediente_id = ANY($1::uuid[])`,
		[]string{ok.exp.ID, ultimoPerdido.exp.ID, retrasado.exp.ID, fallido.exp.ID, divergente.exp.ID, purgado.exp.ID}).Scan(&cuenta); err != nil || cuenta != 8 {
		t.Errorf("movimientos de origen: %d %v", cuenta, err)
	}

	// 4. Segunda republicación: los reencolados ya no son "sin proyección"
	// republicables sino EVENTO_RETRASADO (no se reencolan dos veces).
	rep2, err := Ejecutar(ctx, recExp, recDer, Opciones{Modo: ModoRepublicacion, Gracia: time.Millisecond, MaxRepublicaciones: 1000})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []*repository.Movimiento{ultimoPerdido.movs[1], purgado.movs[0]} {
		if a, hay := rep2.Acciones[m.ID]; hay {
			t.Errorf("segunda republicación no debía actuar sobre %s: %s", m.ID, a)
		}
	}
	if g := filtrar(rep2); g[purgado.exp.ID] != "EVENTO_RETRASADO " || g[ultimoPerdido.exp.ID] != "EVENTO_RETRASADO " {
		t.Errorf("tras republicar: %v", g)
	}

	// 5. Recuperación: se proyecta lo republicado (como haría el consumidor)
	// y la detección ya no lo informa.
	proyectar(ultimoPerdido.movs[1], ultimoPerdido.movs[1].Motivo)
	proyectar(purgado.movs[0], purgado.movs[0].Motivo)
	outbox(ultimoPerdido.movs[1], "PUBLICADO")
	outbox(purgado.movs[0], "PUBLICADO")
	fin, err := Ejecutar(ctx, recExp, recDer, Opciones{Modo: ModoDeteccion, Gracia: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	g := filtrar(fin)
	for _, id := range []string{ok.exp.ID, ultimoPerdido.exp.ID, purgado.exp.ID} {
		if g[id] != "" {
			t.Errorf("tras recuperar, %s aún informa %q", id, g[id])
		}
	}
	var alerta bool
	if err := recExp.QueryRow(ctx, `SELECT alerta_sin_ejecucion FROM reconciliacion_vigilancia`).Scan(&alerta); err != nil || alerta {
		t.Errorf("vigilancia: alerta=%v %v", alerta, err)
	}
	t.Logf("D8: detección %d disc., republicación %d acciones, final %d disc. (ejecuciones %s, %s, %s, %s)",
		len(inf.Discrepancias), len(rep.Acciones), len(fin.Discrepancias), inf.Ejecucion, rep.Ejecucion, rep2.Ejecucion, fin.Ejecucion)
}
