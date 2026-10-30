package integration

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Motor universal de datos contra la BD de PRUEBA (backend/.env.test).
// Datos ficticios con documentos generados por la prueba; todo se limpia.
func TestMotorDatosEstudiantes(t *testing.T) {
	h := newHarness(t)
	suf := fmt.Sprintf("%07d", time.Now().UnixNano()%10_000_000)
	importador, soloCrear, lector := "00000000-0000-4000-8000-0000000d0001", "00000000-0000-4000-8000-0000000d0002", "00000000-0000-4000-8000-0000000d0003"
	h.grant(importador, "estudiantes.create")
	h.grant(importador, "estudiantes.view")
	h.grant(soloCrear, "estudiantes.create")
	h.grant(lector, "estudiantes.view")

	dniExistente, dniNuevo1, dniNuevo2 := "7"+suf, "0"+suf, "6"+suf
	h.seed(seedInput{Codigo: "MD-EX-" + suf, Nombres: "Ana", Apellidos: "Previo", ApellidoPaterno: "Previo", NumeroDocumento: dniExistente, Nivel: "PRIMARIA", Grado: "3", Seccion: "B"})

	// CSV con punto y coma, BOM, columnas reordenadas y una desconocida.
	csv := "\xEF\xBB\xBFNombre(s);Nro. Doc;Ap. Paterno;Teléfono;Condición;Nivel\n" +
		"José;" + dniNuevo1 + ";Núñez;999;ESTUDIANTE;SECUNDARIA\n" +
		"Ana;" + dniExistente + ";Previo;998;ESTUDIANTE;PRIMARIA\n" +
		"Eva;" + dniNuevo2 + ";Díaz;997;DOCENTE;SECUNDARIA\n" +
		";;;;;\n"
	req := func(extra func(*estudiantes.DatosImportacionRequest)) *estudiantes.DatosImportacionRequest {
		r := &estudiantes.DatosImportacionRequest{NombreArchivo: "alumnos.csv", Contenido: []byte(csv)}
		if extra != nil {
			extra(r)
		}
		return r
	}
	contar := func(sql string, args ...any) int {
		var n int
		if err := h.admin.QueryRow(h.ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	registrar := func() {
		rows, _ := h.admin.Query(h.ctx, "SELECT id FROM estudiantes WHERE numero_documento = ANY($1)", []string{dniNuevo1, dniNuevo2, "5" + suf})
		defer rows.Close()
		for rows.Next() {
			var id string
			_ = rows.Scan(&id)
			h.created = append(h.created, id)
		}
	}
	t.Cleanup(registrar)

	t.Run("Permisos", func(t *testing.T) {
		_, err := h.client.AnalizarDatosEstudiantes(h.as(lector), req(nil))
		wantCode(t, err, codes.PermissionDenied, "analizar solo con view")
		_, err = h.client.AnalizarDatosEstudiantes(h.as(soloCrear), req(nil))
		wantCode(t, err, codes.PermissionDenied, "analizar sin view (no puede ver coincidencias)")
		_, err = h.client.ExportarDatosEstudiantes(h.as(soloCrear), &estudiantes.DatosExportacionRequest{})
		wantCode(t, err, codes.PermissionDenied, "exportar sin view")
	})

	var analisis *estudiantes.DatosImportacionResponse
	t.Run("AnalisisSinEscrituras", func(t *testing.T) {
		antes := contar("SELECT count(*) FROM estudiantes")
		var err error
		analisis, err = h.client.AnalizarDatosEstudiantes(h.as(importador), req(nil))
		if err != nil {
			t.Fatal(err)
		}
		if analisis.Formato != "csv" || analisis.Separador != ";" {
			t.Fatalf("formato: %s %s", analisis.Formato, analisis.Separador)
		}
		r := analisis.Resumen
		if r.Total != 3 || r.Nuevos != 1 || r.Existentes != 1 || r.Invalidos != 1 {
			t.Fatalf("resumen: %+v", r)
		}
		if contar("SELECT count(*) FROM estudiantes") != antes {
			t.Fatal("el análisis no debe escribir")
		}
	})

	mapeo := func() []*estudiantes.AsignacionColumna {
		var out []*estudiantes.AsignacionColumna
		for _, c := range analisis.Columnas {
			if c.Campo != "" && c.Importable {
				out = append(out, &estudiantes.AsignacionColumna{Columna: c.Columna, Campo: c.Campo})
			}
		}
		return out
	}
	previa := func() *estudiantes.DatosImportacionResponse {
		p, err := h.client.AnalizarDatosEstudiantes(h.as(importador), req(func(r *estudiantes.DatosImportacionRequest) {
			r.Mapeo, r.MapeoConfirmado = mapeo(), true
		}))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("ConfirmacionExigeVistaPreviaYResolverErrores", func(t *testing.T) {
		_, err := h.client.ConfirmarDatosEstudiantes(h.as(importador), req(nil))
		wantCode(t, err, codes.InvalidArgument, "sin mapeo confirmado ni huella")
		p := previa()
		_, err = h.client.ConfirmarDatosEstudiantes(h.as(importador), req(func(r *estudiantes.DatosImportacionRequest) {
			r.Mapeo, r.MapeoConfirmado, r.Huella = mapeo(), true, p.Huella
		}))
		if status.Code(err) != codes.FailedPrecondition || !strings.Contains(err.Error(), "importacion_con_errores") {
			t.Fatalf("errores sin excluir: %v", err)
		}
		_, err = h.client.ConfirmarDatosEstudiantes(h.as(importador), req(func(r *estudiantes.DatosImportacionRequest) {
			r.Mapeo, r.MapeoConfirmado, r.Huella, r.ExcluirInvalidos = mapeo()[1:], true, p.Huella, true
		}))
		wantCode(t, err, codes.FailedPrecondition, "mapeo distinto del previsualizado (huella)")
	})

	t.Run("AtomicidadAnteConflicto", func(t *testing.T) {
		// Archivo con dos nuevos; antes de confirmar, "otro usuario" registra el segundo.
		dos := "DNI;Apellidos;Nombres\n5" + suf + ";Uno;Lote\n" + dniNuevo2 + ";Dos;Lote\n"
		r := &estudiantes.DatosImportacionRequest{NombreArchivo: "lote.csv", Contenido: []byte(dos)}
		a, err := h.client.AnalizarDatosEstudiantes(h.as(importador), r)
		if err != nil || a.Resumen.Nuevos != 2 {
			t.Fatalf("lote: %+v %v", a.GetResumen(), err)
		}
		var m []*estudiantes.AsignacionColumna
		for _, c := range a.Columnas {
			if c.Campo != "" {
				m = append(m, &estudiantes.AsignacionColumna{Columna: c.Columna, Campo: c.Campo})
			}
		}
		r.Mapeo, r.MapeoConfirmado = m, true
		p, err := h.client.AnalizarDatosEstudiantes(h.as(importador), r)
		if err != nil {
			t.Fatal(err)
		}
		h.seed(seedInput{Codigo: "MD-RACE-" + suf, Nombres: "Otro", Apellidos: "Usuario", NumeroDocumento: dniNuevo2})
		r.Huella = p.Huella
		// La re-validación de la confirmación ya lo detecta como EXISTENTE:
		// solo se registra el primero.
		res, err := h.client.ConfirmarDatosEstudiantes(h.as(importador), r)
		if err != nil {
			t.Fatal(err)
		}
		if res.Resumen.Insertados != 1 || contar("SELECT count(*) FROM estudiantes WHERE numero_documento = $1", dniNuevo2) != 1 {
			t.Fatalf("sin duplicar al registrado mientras tanto: %+v", res.Resumen)
		}
	})

	t.Run("LoteTodoONada", func(t *testing.T) {
		// Fallo DENTRO de la transacción: el segundo choca con un documento
		// existente; el primero no debe quedar guardado.
		primero := "4" + suf
		lote := []repository.Maestro{
			{Codigo: "MD-LOTE1-" + suf, TipoDocumento: "DNI", NumeroDocumento: &primero, Nombres: "Uno", Apellidos: "Lote", Estado: "ACTIVO"},
			{Codigo: "MD-LOTE2-" + suf, TipoDocumento: "DNI", NumeroDocumento: &dniExistente, Nombres: "Dos", Apellidos: "Lote", Estado: "ACTIVO"},
		}
		_, err := h.svcCtx.EstudianteRepository.CrearLote(h.ctx, lote, repository.Auditoria{UsuarioID: importador, Accion: repository.AccionCreate},
			func(repository.Maestro) []string { return []string{"codigo"} })
		var errLote *repository.ErrLote
		if !errors.As(err, &errLote) || errLote.Indice != 1 || !errors.Is(err, repository.ErrDocumentoExists) {
			t.Fatalf("se esperaba el fallo del registro 1 por documento existente: %v", err)
		}
		if contar("SELECT count(*) FROM estudiantes WHERE numero_documento = $1", primero) != 0 {
			t.Fatal("rollback: el primer registro no debe quedar guardado")
		}
	})

	t.Run("ConfirmarYReimportarSinDuplicar", func(t *testing.T) {
		p := previa()
		confirmar := func() *estudiantes.DatosImportacionResponse {
			res, err := h.client.ConfirmarDatosEstudiantes(h.as(importador), req(func(r *estudiantes.DatosImportacionRequest) {
				r.Mapeo, r.MapeoConfirmado, r.Huella, r.ExcluirInvalidos = mapeo(), true, p.Huella, true
			}))
			if err != nil {
				t.Fatal(err)
			}
			return res
		}
		if res := confirmar(); res.Resumen.Insertados != 1 {
			t.Fatalf("primera confirmación: %+v", res.Resumen)
		}
		if contar("SELECT count(*) FROM auditoria_estudiantes a JOIN estudiantes e ON e.id = a.estudiante_id WHERE e.numero_documento = $1 AND a.accion = 'CREATE' AND a.usuario_id = $2", dniNuevo1, importador) != 1 {
			t.Fatal("cada alta queda auditada")
		}
		if res := confirmar(); res.Resumen.Insertados != 0 {
			t.Fatalf("reimportar no duplica: %+v", res.Resumen)
		}
		var nombres, nivel string
		_ = h.admin.QueryRow(h.ctx, "SELECT nombres, nivel FROM estudiantes WHERE numero_documento = $1", dniExistente).Scan(&nombres, &nivel)
		if nombres != "Ana" || nivel != "PRIMARIA" {
			t.Fatal("el existente no se modifica")
		}
	})

	t.Run("ExportarConFiltrosYFormatos", func(t *testing.T) {
		resp, err := h.client.ExportarDatosEstudiantes(h.as(lector), &estudiantes.DatosExportacionRequest{
			Q: "Previo", Formato: "json",
			Columnas: []*estudiantes.ColumnaExportDatos{{Clave: "codigo"}, {Clave: "documento", Encabezado: "Doc"}, {Clave: "grado"}, {Clave: "estado_matricula"}, {Clave: "acciones"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Registros []map[string]string `json:"registros"`
		}
		if err := json.Unmarshal(resp.Contenido, &doc); err != nil {
			t.Fatal(err)
		}
		var hallado bool
		for _, r := range doc.Registros {
			if r["Doc"] == "DNI: "+dniExistente {
				hallado = r["Grado"] == "3" && r["Estado de matrícula"] == "" && r["Acciones"] == ""
			}
		}
		if !hallado {
			t.Fatalf("exportación filtrada (sin matrícula vigente: vacío): %s", resp.Contenido)
		}
		_, err = h.client.ExportarDatosEstudiantes(h.as(lector), &estudiantes.DatosExportacionRequest{Q: "zzzz-no-existe-" + suf})
		wantCode(t, err, codes.FailedPrecondition, "sin resultados")
	})

	t.Run("Plantillas", func(t *testing.T) {
		g, err := h.client.GuardarPlantillaDatos(h.as(importador), &estudiantes.GuardarPlantillaDatosRequest{
			Tipo: "IMPORTACION", Nombre: "Prueba " + suf, Configuracion: `{"asociaciones":{"Nro. Doc":"numero_documento","Nombre(s)":"nombres","Ap. Paterno":"apellido_paterno"}}`,
		})
		if status.Code(err) == codes.FailedPrecondition {
			t.Skip("migración 010 (plantillas_datos) no aplicada en esta base de prueba")
		}
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_, _ = h.client.EliminarPlantillaDatos(h.as(importador), &estudiantes.EliminarPlantillaDatosRequest{Id: g.Plantilla.Id, Tipo: "IMPORTACION"})
		}()
		con, err := h.client.AnalizarDatosEstudiantes(h.as(importador), req(func(r *estudiantes.DatosImportacionRequest) { r.PlantillaId = g.Plantilla.Id }))
		if err != nil || con.PlantillaAplicada == "" {
			t.Fatalf("aplicar plantilla: %v", err)
		}
		_, err = h.client.GuardarPlantillaDatos(h.as(importador), &estudiantes.GuardarPlantillaDatosRequest{
			Id: g.Plantilla.Id, Tipo: "IMPORTACION", Nombre: "x", Configuracion: `{"asociaciones":{"DNI":"numero_documento"}}`, ExpectedVersion: 99,
		})
		wantCode(t, err, codes.Aborted, "versión desactualizada")
		_, err = h.client.GuardarPlantillaDatos(h.as(lector), &estudiantes.GuardarPlantillaDatosRequest{Tipo: "IMPORTACION", Nombre: "y", Configuracion: `{"asociaciones":{"DNI":"numero_documento"}}`})
		wantCode(t, err, codes.PermissionDenied, "plantilla de importación sin permiso de alta")
	})
}
