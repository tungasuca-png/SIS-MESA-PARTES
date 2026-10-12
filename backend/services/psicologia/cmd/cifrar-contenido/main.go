// cifrar-contenido migra el contenido clínico de Psicología de texto claro a
// cifrado (FASE 6N-3, C1) y permite verificarlo o revertirlo.
//
// Uso (una sola acción por ejecución):
//
//	cifrar-contenido --inicializar-canario   crea/verifica el canario de la clave activa
//	cifrar-contenido --huella                huella global del contenido (antes/después)
//	cifrar-contenido --dry-run               cifra y verifica en memoria, sin escribir
//	cifrar-contenido --aplicar               cifra fila por fila (idempotente)
//	cifrar-contenido --verificar             0 pendientes y 100 % descifrable
//	cifrar-contenido --revertir              vuelve a texto claro (solo antes de la 004)
//
// Configuración SOLO por variables de entorno (nunca argumentos):
//
//	PSICOLOGIA_MIGRATOR_DB_URL   postgres://psicologia_migrator:...@host:port/psicologia_db
//	CLINICAL_DATA_ENCRYPTION_KEY, CLINICAL_DATA_KEY_ID, CLINICAL_DATA_PREVIOUS_KEYS
//
// Cada fila se procesa en su propia transacción: SET LOCAL ROLE
// psicologia_owner, SELECT ... FOR UPDATE, cifrar, descifrar y comparar en
// memoria, UPDATE, releer y comparar otra vez, COMMIT. Ante cualquier error la
// fila se revierte y el programa termina con código 1. La salida contiene solo
// tablas, ids y contadores: NUNCA contenido clínico ni claves.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"psicologia/internal/clinicalcrypto"
	"psicologia/internal/repository"

	"github.com/jackc/pgx/v5"
)

const rolDueno = "psicologia_owner"

// Columnas en claro de atencion_versiones, en el orden de ContenidoCifrable.
var columnasClaroVersion = []string{"realizada_por_otros", "motivo_otros", "descripcion_motivo", "analisis_integral",
	"estrategias", "derivacion_otro", "seguimiento_resultados", "acuerdos", "compromiso_estudiante",
	"compromiso_familia", "motivo_correccion"}

type herramienta struct {
	ctx  context.Context
	conn *pgx.Conn
	c    *clinicalcrypto.Cipher
	dry  bool
}

func main() {
	var inicializar, huella, dry, aplicar, verificar, revertir bool
	flag.BoolVar(&inicializar, "inicializar-canario", false, "crea o verifica el canario de la clave activa")
	flag.BoolVar(&huella, "huella", false, "huella global del contenido clínico")
	flag.BoolVar(&dry, "dry-run", false, "cifra y verifica en memoria sin escribir")
	flag.BoolVar(&aplicar, "aplicar", false, "cifra los datos existentes")
	flag.BoolVar(&verificar, "verificar", false, "verifica 0 pendientes y 100 % descifrable")
	flag.BoolVar(&revertir, "revertir", false, "vuelve a texto claro (solo antes de la migración 004)")
	flag.Parse()
	n := 0
	for _, b := range []bool{inicializar, huella, dry, aplicar, verificar, revertir} {
		if b {
			n++
		}
	}
	if n != 1 {
		fallar("indicar exactamente una acción (--help)")
	}

	dsn := os.Getenv("PSICOLOGIA_MIGRATOR_DB_URL")
	if dsn == "" {
		fallar("falta PSICOLOGIA_MIGRATOR_DB_URL")
	}
	c, err := clinicalcrypto.New(os.Getenv("CLINICAL_DATA_ENCRYPTION_KEY"), os.Getenv("CLINICAL_DATA_KEY_ID"), os.Getenv("CLINICAL_DATA_PREVIOUS_KEYS"))
	if err != nil {
		fallar(err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		fallar("no se pudo conectar a la base (revisar PSICOLOGIA_MIGRATOR_DB_URL)")
	}
	defer conn.Close(ctx)
	// El migrador es miembro de psicologia_owner SIN herencia: toda la sesión
	// (lecturas incluidas) actúa como dueño de forma explícita.
	if _, err := conn.Exec(ctx, "SET ROLE "+rolDueno); err != nil {
		fallar("no se pudo actuar como " + rolDueno + " (¿el usuario es psicologia_migrator?)")
	}
	h := &herramienta{ctx: ctx, conn: conn, c: c, dry: dry}

	switch {
	case inicializar:
		err = h.inicializarCanario()
	case huella:
		err = h.huella()
	case dry, aplicar:
		if err = h.exigirCanario(); err == nil {
			err = h.cifrarTodo()
		}
	case verificar:
		err = h.verificar()
	case revertir:
		if err = h.exigirCanario(); err == nil {
			err = h.revertirTodo()
		}
	}
	if err != nil {
		fallar(err.Error())
	}
}

func fallar(msg string) {
	fmt.Fprintln(os.Stderr, "ERROR:", msg)
	os.Exit(1)
}

// enTx ejecuta fn en una transacción actuando como dueño (el trigger de
// inmutabilidad solo admite la transición de cifrado a psicologia_owner). En
// dry-run siempre se revierte.
func (h *herramienta) enTx(fn func(tx pgx.Tx) error) error {
	tx, err := h.conn.Begin(h.ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(h.ctx)
	if _, err := tx.Exec(h.ctx, "SET LOCAL ROLE "+rolDueno); err != nil {
		return fmt.Errorf("no se pudo actuar como %s: %w", rolDueno, err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	if h.dry {
		return nil // Rollback diferido: dry-run no escribe
	}
	return tx.Commit(h.ctx)
}

func (h *herramienta) inicializarCanario() error {
	return h.enTx(func(tx pgx.Tx) error {
		creado, err := repository.InicializarCanario(h.ctx, tx, h.c)
		if err != nil {
			return err
		}
		if creado {
			fmt.Printf("canario creado para la clave %q\n", h.c.ActiveKeyID())
		} else {
			fmt.Printf("canario de la clave %q ya existía y se verificó\n", h.c.ActiveKeyID())
		}
		return nil
	})
}

func (h *herramienta) exigirCanario() error {
	if err := repository.VerificarCanario(h.ctx, h.conn, h.c); err != nil {
		return fmt.Errorf("%w (ejecutar primero --inicializar-canario)", err)
	}
	return nil
}

func (h *herramienta) columnaExiste(tabla, columna string) (bool, error) {
	var ok bool
	err := h.conn.QueryRow(h.ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2)`, tabla, columna).Scan(&ok)
	return ok, err
}

func (h *herramienta) ids(q string) ([]string, error) {
	rows, err := h.conn.Query(h.ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func texto(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nulo(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ---------------------------------------------------------------------------
// Cifrado de los datos existentes
// ---------------------------------------------------------------------------

func (h *herramienta) cifrarTodo() error {
	modo := "APLICAR"
	if h.dry {
		modo = "DRY-RUN (sin escribir)"
	}
	fmt.Println("modo:", modo, "— clave activa:", h.c.ActiveKeyID())

	ids, err := h.ids(`SELECT id::text FROM atencion_versiones WHERE contenido_cifrado IS NULL ORDER BY id`)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := h.enTx(func(tx pgx.Tx) error { return h.cifrarVersion(tx, id) }); err != nil {
			return fmt.Errorf("atencion_versiones %s: %w", id, err)
		}
	}
	fmt.Printf("atencion_versiones: %d fila(s) cifradas y verificadas\n", len(ids))

	ids, err = h.ids(`SELECT id::text FROM atenciones_psicologicas
		WHERE motivo_anulacion IS NOT NULL AND motivo_anulacion_cifrado IS NULL ORDER BY id`)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := h.enTx(func(tx pgx.Tx) error { return h.cifrarAnulacion(tx, id) }); err != nil {
			return fmt.Errorf("atenciones_psicologicas %s: %w", id, err)
		}
	}
	fmt.Printf("atenciones_psicologicas (motivo_anulacion): %d fila(s) cifradas y verificadas\n", len(ids))

	ids, err = h.ids(`SELECT id::text FROM expedientes_psicologicos
		WHERE (resumen_caso IS NOT NULL OR motivo_cierre IS NOT NULL)
		  AND resumen_caso_cifrado IS NULL AND motivo_cierre_cifrado IS NULL ORDER BY id`)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := h.enTx(func(tx pgx.Tx) error { return h.cifrarExpediente(tx, id) }); err != nil {
			return fmt.Errorf("expedientes_psicologicos %s: %w", id, err)
		}
	}
	fmt.Printf("expedientes_psicologicos (resumen_caso, motivo_cierre): %d fila(s) cifradas y verificadas\n", len(ids))
	return nil
}

func (h *herramienta) cifrarVersion(tx pgx.Tx, id string) error {
	var atencionID string
	var v [11]*string
	dest := []any{&atencionID}
	for i := range v {
		dest = append(dest, &v[i])
	}
	err := tx.QueryRow(h.ctx, `SELECT atencion_id::text, `+strings.Join(columnasClaroVersion, ", ")+`
		FROM atencion_versiones WHERE id = $1::uuid AND contenido_cifrado IS NULL FOR UPDATE`, id).Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // ya cifrada por otra ejecución (idempotente)
	}
	if err != nil {
		return err
	}
	cc := repository.ContenidoCifrable{
		RealizadaPorOtros: texto(v[0]), MotivoOtros: texto(v[1]), DescripcionMotivo: texto(v[2]),
		AnalisisIntegral: texto(v[3]), Estrategias: texto(v[4]), DerivacionOtro: texto(v[5]),
		SeguimientoResultados: texto(v[6]), Acuerdos: texto(v[7]), CompromisoEstudiante: texto(v[8]),
		CompromisoFamilia: texto(v[9]), MotivoCorreccion: texto(v[10]),
	}
	blob, kid, err := repository.CifrarContenidoVersion(h.c, id, atencionID, cc)
	if err != nil {
		return err
	}
	if back, err := repository.DescifrarContenidoVersion(h.c, blob, kid, id, atencionID); err != nil || back != cc {
		return errors.New("la verificación en memoria no coincide con el original")
	}
	sets := make([]string, 0, len(columnasClaroVersion))
	for _, col := range columnasClaroVersion {
		sets = append(sets, col+" = NULL")
	}
	if _, err := tx.Exec(h.ctx, `UPDATE atencion_versiones SET contenido_cifrado = $2, cifrado_key_id = $3, `+
		strings.Join(sets, ", ")+` WHERE id = $1::uuid`, id, blob, kid); err != nil {
		return err
	}
	var guardado []byte
	var guardadoKid string
	if err := tx.QueryRow(h.ctx, `SELECT contenido_cifrado, cifrado_key_id FROM atencion_versiones WHERE id = $1::uuid`, id).Scan(&guardado, &guardadoKid); err != nil {
		return err
	}
	if back, err := repository.DescifrarContenidoVersion(h.c, guardado, guardadoKid, id, atencionID); err != nil || back != cc {
		return errors.New("lo guardado no descifra al contenido original")
	}
	return nil
}

func (h *herramienta) cifrarTextoFila(tx pgx.Tx, tabla, id, col, colCifrada string, aad []byte, extraSet string) error {
	var valor *string
	err := tx.QueryRow(h.ctx, `SELECT `+col+` FROM `+tabla+` WHERE id = $1::uuid AND `+colCifrada+` IS NULL FOR UPDATE`, id).Scan(&valor)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // ya cifrado por otra ejecución (idempotente)
	}
	if err != nil {
		return err
	}
	if valor == nil {
		return nil // campo vacío: no hay nada que cifrar
	}
	blob, kid, err := h.c.Encrypt([]byte(*valor), aad)
	if err != nil {
		return err
	}
	if back, err := h.c.Decrypt(blob, kid, aad); err != nil || string(back) != *valor {
		return errors.New("la verificación en memoria no coincide con el original")
	}
	if _, err := tx.Exec(h.ctx, `UPDATE `+tabla+` SET `+colCifrada+` = $2, cifrado_key_id = $3, `+col+` = NULL`+extraSet+
		` WHERE id = $1::uuid`, id, blob, kid); err != nil {
		return err
	}
	var guardado []byte
	if err := tx.QueryRow(h.ctx, `SELECT `+colCifrada+` FROM `+tabla+` WHERE id = $1::uuid`, id).Scan(&guardado); err != nil {
		return err
	}
	if back, err := h.c.Decrypt(guardado, kid, aad); err != nil || string(back) != *valor {
		return errors.New("lo guardado no descifra al contenido original")
	}
	return nil
}

func (h *herramienta) cifrarAnulacion(tx pgx.Tx, id string) error {
	return h.cifrarTextoFila(tx, "atenciones_psicologicas", id, "motivo_anulacion", "motivo_anulacion_cifrado",
		clinicalcrypto.AADMotivoAnulacion(id), "")
}

func (h *herramienta) cifrarExpediente(tx pgx.Tx, id string) error {
	if err := h.cifrarTextoFila(tx, "expedientes_psicologicos", id, "resumen_caso", "resumen_caso_cifrado",
		clinicalcrypto.AADResumenCaso(id), ""); err != nil {
		return err
	}
	return h.cifrarTextoFila(tx, "expedientes_psicologicos", id, "motivo_cierre", "motivo_cierre_cifrado",
		clinicalcrypto.AADMotivoCierre(id), "")
}

// ---------------------------------------------------------------------------
// Verificación
// ---------------------------------------------------------------------------

func (h *herramienta) verificar() error {
	problemas := 0
	contar := func(nombre, q string) {
		var n int
		if err := h.conn.QueryRow(h.ctx, q).Scan(&n); err != nil {
			fmt.Printf("%-55s ERROR al consultar\n", nombre)
			problemas++
			return
		}
		estado := "OK"
		if n != 0 {
			estado = "PENDIENTE"
			problemas++
		}
		fmt.Printf("%-55s %d  %s\n", nombre, n, estado)
	}
	contar("versiones sin contenido_cifrado", `SELECT COUNT(*) FROM atencion_versiones WHERE contenido_cifrado IS NULL`)
	contar("atenciones ANULADAS sin motivo cifrado", `SELECT COUNT(*) FROM atenciones_psicologicas WHERE estado = 'ANULADA' AND motivo_anulacion_cifrado IS NULL`)
	contar("expedientes CERRADOS sin motivo de cierre cifrado", `SELECT COUNT(*) FROM expedientes_psicologicos WHERE estado = 'CERRADO' AND motivo_cierre_cifrado IS NULL`)

	// Texto claro residual (mientras existan las columnas, antes de la 004).
	if ok, _ := h.columnaExiste("atencion_versiones", "acuerdos"); ok {
		conds := make([]string, 0, len(columnasClaroVersion))
		for _, c := range columnasClaroVersion {
			conds = append(conds, c+" IS NOT NULL")
		}
		contar("versiones con texto claro residual", `SELECT COUNT(*) FROM atencion_versiones WHERE `+strings.Join(conds, " OR "))
		contar("atenciones con motivo_anulacion en claro", `SELECT COUNT(*) FROM atenciones_psicologicas WHERE motivo_anulacion IS NOT NULL`)
		contar("expedientes con resumen/motivo en claro", `SELECT COUNT(*) FROM expedientes_psicologicos WHERE resumen_caso IS NOT NULL OR motivo_cierre IS NOT NULL`)
	} else {
		fmt.Println("columnas en claro: ELIMINADAS (migración 004 aplicada)")
	}

	ok, fallas, err := h.descifrarTodo(nil)
	if err != nil {
		return err
	}
	fmt.Printf("%-55s %d descifrados, %d fallidos\n", "bloques cifrados", ok, fallas)
	if fallas > 0 {
		problemas++
	}
	if problemas > 0 {
		return fmt.Errorf("verificación con %d problema(s)", problemas)
	}
	fmt.Println("VERIFICACIÓN: OK (0 pendientes, 100 % descifrable)")
	return nil
}

// descifrarTodo descifra todos los bloques; si huella != nil, agrega cada
// contenido descifrado (id|json) para la huella global.
func (h *herramienta) descifrarTodo(huella map[string]string) (ok, fallas int, err error) {
	rows, err := h.conn.Query(h.ctx, `SELECT id::text, atencion_id::text, contenido_cifrado, cifrado_key_id FROM atencion_versiones WHERE contenido_cifrado IS NOT NULL`)
	if err != nil {
		return 0, 0, err
	}
	type fila struct {
		id, padre string
		blob      []byte
		kid       string
	}
	var vs []fila
	for rows.Next() {
		var f fila
		if err := rows.Scan(&f.id, &f.padre, &f.blob, &f.kid); err != nil {
			rows.Close()
			return 0, 0, err
		}
		vs = append(vs, f)
	}
	rows.Close()
	for _, f := range vs {
		cc, err := repository.DescifrarContenidoVersion(h.c, f.blob, f.kid, f.id, f.padre)
		if err != nil {
			fmt.Printf("  no descifra: atencion_versiones %s\n", f.id)
			fallas++
			continue
		}
		ok++
		if huella != nil {
			j, _ := json.Marshal(cc)
			huella["v/"+f.id] = string(j)
		}
	}

	textos := []struct{ tabla, col, cifrada, prefijo string }{
		{"atenciones_psicologicas", "motivo_anulacion", "motivo_anulacion_cifrado", "a/"},
		{"expedientes_psicologicos", "resumen_caso", "resumen_caso_cifrado", "er/"},
		{"expedientes_psicologicos", "motivo_cierre", "motivo_cierre_cifrado", "em/"},
	}
	for _, t := range textos {
		rows, err := h.conn.Query(h.ctx, `SELECT id::text, `+t.cifrada+`, cifrado_key_id FROM `+t.tabla+` WHERE `+t.cifrada+` IS NOT NULL`)
		if err != nil {
			return 0, 0, err
		}
		type fila2 struct {
			id   string
			blob []byte
			kid  string
		}
		var fs []fila2
		for rows.Next() {
			var f fila2
			if err := rows.Scan(&f.id, &f.blob, &f.kid); err != nil {
				rows.Close()
				return 0, 0, err
			}
			fs = append(fs, f)
		}
		rows.Close()
		for _, f := range fs {
			var aad []byte
			switch t.prefijo {
			case "a/":
				aad = clinicalcrypto.AADMotivoAnulacion(f.id)
			case "er/":
				aad = clinicalcrypto.AADResumenCaso(f.id)
			default:
				aad = clinicalcrypto.AADMotivoCierre(f.id)
			}
			plano, err := h.c.Decrypt(f.blob, f.kid, aad)
			if err != nil {
				fmt.Printf("  no descifra: %s.%s %s\n", t.tabla, t.cifrada, f.id)
				fallas++
				continue
			}
			ok++
			if huella != nil {
				huella[t.prefijo+f.id] = string(plano)
			}
		}
	}
	return ok, fallas, nil
}

// huella: SHA-256 de todo el contenido clínico (id + contenido), tomado del
// texto claro o del bloque descifrado según el estado de cada fila. Debe ser
// idéntica antes y después de cifrar. Solo se imprime el resumen, nunca el
// contenido.
func (h *herramienta) huella() error {
	datos := map[string]string{}
	if ok, _ := h.columnaExiste("atencion_versiones", "acuerdos"); ok {
		rows, err := h.conn.Query(h.ctx, `SELECT id::text, `+strings.Join(columnasClaroVersion, ", ")+`
			FROM atencion_versiones WHERE contenido_cifrado IS NULL`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			var v [11]*string
			dest := []any{&id}
			for i := range v {
				dest = append(dest, &v[i])
			}
			if err := rows.Scan(dest...); err != nil {
				rows.Close()
				return err
			}
			cc := repository.ContenidoCifrable{
				RealizadaPorOtros: texto(v[0]), MotivoOtros: texto(v[1]), DescripcionMotivo: texto(v[2]),
				AnalisisIntegral: texto(v[3]), Estrategias: texto(v[4]), DerivacionOtro: texto(v[5]),
				SeguimientoResultados: texto(v[6]), Acuerdos: texto(v[7]), CompromisoEstudiante: texto(v[8]),
				CompromisoFamilia: texto(v[9]), MotivoCorreccion: texto(v[10]),
			}
			j, _ := json.Marshal(cc)
			datos["v/"+id] = string(j)
		}
		rows.Close()
		for _, t := range []struct{ tabla, col, cifrada, prefijo string }{
			{"atenciones_psicologicas", "motivo_anulacion", "motivo_anulacion_cifrado", "a/"},
			{"expedientes_psicologicos", "resumen_caso", "resumen_caso_cifrado", "er/"},
			{"expedientes_psicologicos", "motivo_cierre", "motivo_cierre_cifrado", "em/"},
		} {
			rows, err := h.conn.Query(h.ctx, `SELECT id::text, `+t.col+` FROM `+t.tabla+` WHERE `+t.col+` IS NOT NULL AND `+t.cifrada+` IS NULL`)
			if err != nil {
				return err
			}
			for rows.Next() {
				var id, v string
				if err := rows.Scan(&id, &v); err != nil {
					rows.Close()
					return err
				}
				if v != "" {
					datos[t.prefijo+id] = v
				}
			}
			rows.Close()
		}
	}
	if err := h.exigirCanarioSiHayCifrado(); err != nil {
		return err
	}
	if _, fallas, err := h.descifrarTodo(datos); err != nil {
		return err
	} else if fallas > 0 {
		return fmt.Errorf("%d bloque(s) no descifran: la huella no es confiable", fallas)
	}
	claves := make([]string, 0, len(datos))
	for k := range datos {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	sum := sha256.New()
	for _, k := range claves {
		sum.Write([]byte(k + "\x00" + datos[k] + "\x00"))
	}
	fmt.Printf("huella: %s (%d elementos de contenido clínico)\n", hex.EncodeToString(sum.Sum(nil)), len(claves))
	return nil
}

func (h *herramienta) exigirCanarioSiHayCifrado() error {
	var n int
	if err := h.conn.QueryRow(h.ctx, `SELECT COUNT(*) FROM atencion_versiones WHERE contenido_cifrado IS NOT NULL`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	return h.exigirCanario()
}

// ---------------------------------------------------------------------------
// Reversión (solo antes de la migración 004)
// ---------------------------------------------------------------------------

func (h *herramienta) revertirTodo() error {
	if ok, _ := h.columnaExiste("atencion_versiones", "acuerdos"); !ok {
		return errors.New("las columnas en claro ya no existen (004 aplicada): solo queda restaurar el backup")
	}
	ids, err := h.ids(`SELECT id::text FROM atencion_versiones WHERE contenido_cifrado IS NOT NULL ORDER BY id`)
	if err != nil {
		return err
	}
	for _, id := range ids {
		err := h.enTx(func(tx pgx.Tx) error {
			var padre, kid string
			var blob []byte
			if err := tx.QueryRow(h.ctx, `SELECT atencion_id::text, contenido_cifrado, cifrado_key_id FROM atencion_versiones
				WHERE id = $1::uuid AND contenido_cifrado IS NOT NULL FOR UPDATE`, id).Scan(&padre, &blob, &kid); err != nil {
				return err
			}
			cc, err := repository.DescifrarContenidoVersion(h.c, blob, kid, id, padre)
			if err != nil {
				return err
			}
			_, err = tx.Exec(h.ctx, `UPDATE atencion_versiones SET contenido_cifrado = NULL, cifrado_key_id = NULL,
				realizada_por_otros = $2, motivo_otros = $3, descripcion_motivo = $4, analisis_integral = $5,
				estrategias = $6, derivacion_otro = $7, seguimiento_resultados = $8, acuerdos = $9,
				compromiso_estudiante = $10, compromiso_familia = $11, motivo_correccion = $12
				WHERE id = $1::uuid`, id, nulo(cc.RealizadaPorOtros), nulo(cc.MotivoOtros), nulo(cc.DescripcionMotivo),
				nulo(cc.AnalisisIntegral), nulo(cc.Estrategias), nulo(cc.DerivacionOtro), nulo(cc.SeguimientoResultados),
				nulo(cc.Acuerdos), nulo(cc.CompromisoEstudiante), nulo(cc.CompromisoFamilia), nulo(cc.MotivoCorreccion))
			return err
		})
		if err != nil {
			return fmt.Errorf("atencion_versiones %s: %w", id, err)
		}
	}
	fmt.Printf("atencion_versiones: %d fila(s) revertidas a texto claro\n", len(ids))

	revertirTexto := func(tabla, col, cifrada string, aad func(string) []byte) (int, error) {
		ids, err := h.ids(`SELECT id::text FROM ` + tabla + ` WHERE ` + cifrada + ` IS NOT NULL ORDER BY id`)
		if err != nil {
			return 0, err
		}
		for _, id := range ids {
			err := h.enTx(func(tx pgx.Tx) error {
				var blob []byte
				var kid string
				if err := tx.QueryRow(h.ctx, `SELECT `+cifrada+`, cifrado_key_id FROM `+tabla+` WHERE id = $1::uuid FOR UPDATE`, id).Scan(&blob, &kid); err != nil {
					return err
				}
				plano, err := h.c.Decrypt(blob, kid, aad(id))
				if err != nil {
					return err
				}
				_, err = tx.Exec(h.ctx, `UPDATE `+tabla+` SET `+col+` = $2, `+cifrada+` = NULL WHERE id = $1::uuid`, id, string(plano))
				return err
			})
			if err != nil {
				return 0, fmt.Errorf("%s %s: %w", tabla, id, err)
			}
		}
		return len(ids), nil
	}
	n, err := revertirTexto("atenciones_psicologicas", "motivo_anulacion", "motivo_anulacion_cifrado", clinicalcrypto.AADMotivoAnulacion)
	if err != nil {
		return err
	}
	fmt.Printf("atenciones_psicologicas: %d motivo(s) de anulación revertidos\n", n)
	n1, err := revertirTexto("expedientes_psicologicos", "resumen_caso", "resumen_caso_cifrado", clinicalcrypto.AADResumenCaso)
	if err != nil {
		return err
	}
	n2, err := revertirTexto("expedientes_psicologicos", "motivo_cierre", "motivo_cierre_cifrado", clinicalcrypto.AADMotivoCierre)
	if err != nil {
		return err
	}
	fmt.Printf("expedientes_psicologicos: %d resumen(es) y %d motivo(s) de cierre revertidos\n", n1, n2)
	// cifrado_key_id queda sin uso cuando ya no hay bloques.
	for _, tabla := range []string{"atenciones_psicologicas", "expedientes_psicologicos"} {
		if err := h.enTx(func(tx pgx.Tx) error {
			_, err := tx.Exec(h.ctx, `UPDATE `+tabla+` SET cifrado_key_id = NULL WHERE cifrado_key_id IS NOT NULL
				AND `+map[string]string{
				"atenciones_psicologicas":  "motivo_anulacion_cifrado IS NULL",
				"expedientes_psicologicos": "resumen_caso_cifrado IS NULL AND motivo_cierre_cifrado IS NULL",
			}[tabla])
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}
