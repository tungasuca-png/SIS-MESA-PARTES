package integration

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"estudiantes/estudiantes"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestIdentidadMigracion002 verifica en PostgreSQL real las reglas de identidad
// de la migración 002 (FASE E-03): documento por tipo, unicidades, campos
// nuevos, updated_at/version y compatibilidad de los RPC existentes. Inserta
// con el rol de la aplicación (estudiantes_user) y limpia con el de pruebas.
func TestIdentidadMigracion002(t *testing.T) {
	h := newHarness(t)
	base := time.Now().UnixNano()
	n := 0
	codigo := func() string { n++; return fmt.Sprintf("E03-%d-%d", base%1_000_000_000, n) }
	dni := func(i int) string { return fmt.Sprintf("6%07d", (base/1000+int64(i))%10_000_000) }

	insertar := func(extraCols string, extraVals ...any) (string, error) {
		cols := "codigo, nombres, apellidos"
		vals := []any{codigo(), "Prueba", "Identidad"}
		ph := "$1, $2, $3"
		if extraCols != "" {
			cols += ", " + extraCols
			for i := range extraVals {
				ph += fmt.Sprintf(", $%d", i+4)
			}
			vals = append(vals, extraVals...)
		}
		var id string
		err := h.app.QueryRow(h.ctx, "INSERT INTO estudiantes ("+cols+") VALUES ("+ph+") RETURNING id", vals...).Scan(&id)
		if err == nil {
			h.created = append(h.created, id)
		}
		return id, err
	}
	sqlstate := func(err error) string {
		var pg *pgconn.PgError
		if errors.As(err, &pg) {
			return pg.Code
		}
		return ""
	}
	debeFallar := func(t *testing.T, err error, codigoSQL, que string) {
		t.Helper()
		if sqlstate(err) != codigoSQL {
			t.Fatalf("%s: se esperaba SQLSTATE %s, se obtuvo %v", que, codigoSQL, err)
		}
	}

	t.Run("SinDocumento_VariosPermitidos_YNumeroNulo", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			if _, err := insertar("", nil...); err != nil {
				t.Fatalf("estudiante sin documento #%d: %v", i+1, err)
			}
		}
		_, err := insertar("tipo_documento, numero_documento", "SIN_DOCUMENTO", dni(1))
		debeFallar(t, err, "23514", "SIN_DOCUMENTO con número")
		_, err = insertar("tipo_documento", "DNI")
		debeFallar(t, err, "23514", "DNI sin número")
	})

	t.Run("DNI_Formato_YDuplicadoRechazado", func(t *testing.T) {
		if _, err := insertar("tipo_documento, numero_documento", "DNI", dni(2)); err != nil {
			t.Fatalf("DNI válido: %v", err)
		}
		_, err := insertar("tipo_documento, numero_documento", "DNI", dni(2))
		debeFallar(t, err, "23505", "DNI duplicado")
		for _, malo := range []string{"1234567", "123456789", "1234567A"} {
			_, err = insertar("tipo_documento, numero_documento", "DNI", malo)
			debeFallar(t, err, "23514", "DNI con formato inválido")
		}
	})

	t.Run("MismoNumero_DistintoTipo_Permitido", func(t *testing.T) {
		num := dni(3)
		if _, err := insertar("tipo_documento, numero_documento", "DNI", num); err != nil {
			t.Fatalf("DNI: %v", err)
		}
		if _, err := insertar("tipo_documento, numero_documento", "CE", num); err != nil {
			t.Fatalf("mismo número como CE debe permitirse (unicidad por tipo+número): %v", err)
		}
		_, err := insertar("tipo_documento, numero_documento", "CE", num)
		debeFallar(t, err, "23505", "CE duplicado")
	})

	t.Run("CE_Pasaporte_Alfanumerico", func(t *testing.T) {
		if _, err := insertar("tipo_documento, numero_documento", "PASAPORTE", fmt.Sprintf("PA%07d", base%10_000_000)); err != nil {
			t.Fatalf("pasaporte alfanumérico: %v", err)
		}
		_, err := insertar("tipo_documento, numero_documento", "CE", "ab12")
		debeFallar(t, err, "23514", "CE en minúsculas")
		// Un tipo fuera del catálogo lo rechaza el CHECK (23514) o la FK (23503):
		// cualquiera de los dos es correcto, lo que importa es que no entre.
		_, err = insertar("tipo_documento, numero_documento", "OTRO", "X1234")
		if c := sqlstate(err); c != "23514" && c != "23503" {
			t.Fatalf("tipo de documento fuera del catálogo debe rechazarse, se obtuvo %v", err)
		}
	})

	t.Run("CodigoSiagie_UnicoCuandoExiste", func(t *testing.T) {
		siagie := fmt.Sprintf("%014d", base%100_000_000_000_000)
		if _, err := insertar("codigo_siagie", siagie); err != nil {
			t.Fatalf("código SIAGIE: %v", err)
		}
		_, err := insertar("codigo_siagie", siagie)
		debeFallar(t, err, "23505", "código SIAGIE duplicado")
		_, err = insertar("codigo_siagie", "123")
		debeFallar(t, err, "23514", "código SIAGIE con formato inválido")
		for i := 0; i < 2; i++ { // varios sin código SIAGIE
			if _, err := insertar("", nil...); err != nil {
				t.Fatalf("sin código SIAGIE: %v", err)
			}
		}
	})

	t.Run("UpdatedAt_YVersion", func(t *testing.T) {
		id, err := insertar("", nil...)
		if err != nil {
			t.Fatal(err)
		}
		var v1 int
		var u1 time.Time
		if err := h.app.QueryRow(h.ctx, `SELECT version, updated_at FROM estudiantes WHERE id = $1`, id).Scan(&v1, &u1); err != nil {
			t.Fatal(err)
		}
		if v1 != 1 {
			t.Fatalf("version debe empezar en 1, es %d", v1)
		}
		time.Sleep(20 * time.Millisecond)
		// La aplicación intenta fijar version/updated_at: el trigger manda.
		if _, err := h.app.Exec(h.ctx, `UPDATE estudiantes SET nombres = 'Editado', version = 99,
			updated_at = '2000-01-01' WHERE id = $1`, id); err != nil {
			t.Fatal(err)
		}
		var v2 int
		var u2 time.Time
		if err := h.app.QueryRow(h.ctx, `SELECT version, updated_at FROM estudiantes WHERE id = $1`, id).Scan(&v2, &u2); err != nil {
			t.Fatal(err)
		}
		if v2 != 2 || !u2.After(u1) {
			t.Fatalf("tras un UPDATE: version=%d (esperado 2), updated_at avanzó=%v", v2, u2.After(u1))
		}
	})

	// fila inserta con columnas explícitas (codigo siempre generado).
	fila := func(cols []string, vals []any) (string, error) {
		allCols := append([]string{"codigo"}, cols...)
		allVals := append([]any{codigo()}, vals...)
		ph := make([]string, len(allVals))
		for i := range ph {
			ph[i] = fmt.Sprintf("$%d", i+1)
		}
		var id string
		err := h.app.QueryRow(h.ctx, "INSERT INTO estudiantes ("+strings.Join(allCols, ", ")+") VALUES ("+strings.Join(ph, ", ")+") RETURNING id", allVals...).Scan(&id)
		if err == nil {
			h.created = append(h.created, id)
		}
		return id, err
	}

	t.Run("CamposNuevos_Reglas", func(t *testing.T) {
		id, err := fila(
			[]string{"nombres", "apellidos", "apellido_paterno", "apellido_materno", "sexo", "pais_nacimiento", "ubigeo_nacimiento", "lengua_materna", "segunda_lengua", "foto_ref"},
			[]any{"Ana", "DE LA CRUZ QUISPE", "DE LA CRUZ", "QUISPE", "F", "PE", "150101", "CASTELLANO", "QUECHUA", "fotos/abc.jpg"})
		if err != nil {
			t.Fatalf("identidad completa válida: %v", err)
		}
		casos := []struct {
			que  string
			cols []string
			vals []any
		}{
			{"apellidos que no coinciden con los separados", []string{"nombres", "apellidos", "apellido_paterno"}, []any{"X", "PEREZ LOPEZ", "GARCIA"}},
			{"sexo fuera de M/F", []string{"nombres", "apellidos", "sexo"}, []any{"X", "Y", "X"}},
			{"ubigeo con país distinto de PE", []string{"nombres", "apellidos", "pais_nacimiento", "ubigeo_nacimiento"}, []any{"X", "Y", "CL", "150101"}},
			{"ubigeo sin país", []string{"nombres", "apellidos", "ubigeo_nacimiento"}, []any{"X", "Y", "150101"}},
			{"CE sin número", []string{"nombres", "apellidos", "tipo_documento"}, []any{"X", "Y", "CE"}},
			{"segunda lengua igual a la materna", []string{"nombres", "apellidos", "lengua_materna", "segunda_lengua"}, []any{"X", "Y", "QUECHUA", "QUECHUA"}},
			{"foto como URL pública", []string{"nombres", "apellidos", "foto_ref"}, []any{"X", "Y", "https://ejemplo.com/foto.jpg"}},
			{"estado fuera de ACTIVO/INACTIVO", []string{"nombres", "apellidos", "estado"}, []any{"X", "Y", "RETIRADO"}},
		}
		for _, c := range casos {
			_, err := fila(c.cols, c.vals)
			debeFallar(t, err, "23514", c.que)
		}
		var estado string
		var version int
		if err := h.app.QueryRow(h.ctx, `SELECT estado, version FROM estudiantes WHERE id = $1`, id).Scan(&estado, &version); err != nil || estado != "ACTIVO" || version != 1 {
			t.Fatalf("valores por defecto: estado=%q version=%d err=%v", estado, version, err)
		}
	})

	t.Run("RPC_Compatibles_ConEstudianteSinDocumento", func(t *testing.T) {
		actor := "00000000-0000-4000-8000-000000000003"
		h.grant(actor, "estudiantes.view")
		id := h.seed(seedInput{Codigo: codigo(), Nombres: "Sin", Apellidos: "Documento"})
		got, err := h.client.GetEstudiante(h.as(actor), &estudiantes.GetEstudianteRequest{Id: id})
		if err != nil || got.Estudiante.Id != id {
			t.Fatalf("GetEstudiante sin documento: %v", err)
		}
		basic, err := h.client.GetEstudiantesBasic(h.as(actor), &estudiantes.GetEstudiantesBasicRequest{Ids: []string{id}})
		if err != nil || len(basic.Estudiantes) != 1 || basic.Estudiantes[0].NumeroDocumento != "" {
			t.Fatalf("GetEstudiantesBasic debe devolver numero_documento vacío: %+v err=%v", basic, err)
		}
		busq, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "Documento"})
		if err != nil || busq.Total < 1 {
			t.Fatalf("SearchEstudiantes: %+v err=%v", busq, err)
		}
	})
}
