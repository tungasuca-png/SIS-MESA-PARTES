package integration

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"estudiantes/estudiantes"
	"estudiantes/internal/excel"

	"github.com/xuri/excelize/v2"
	"google.golang.org/grpc/codes"
)

// Importación/exportación Excel contra la BD de PRUEBA (estudiantes_db con
// las credenciales de backend/.env.test). Datos ficticios con números de
// documento generados por la prueba; todo lo creado se limpia al terminar.
func TestImportacionExportacionExcel(t *testing.T) {
	h := newHarness(t)
	sufijo := fmt.Sprintf("%07d", time.Now().UnixNano()%10_000_000)
	importador, lector, nadie := "00000000-0000-4000-8000-0000000e1001", "00000000-0000-4000-8000-0000000e1002", "00000000-0000-4000-8000-0000000e1003"
	h.grant(importador, "estudiantes.create")
	h.grant(importador, "estudiantes.view")
	h.grant(lector, "estudiantes.view")

	dniExistente := "8" + sufijo
	dniNuevo := "0" + sufijo // cero inicial
	ce := "CE" + sufijo
	nombreSD := "Sindoc" + sufijo
	existente := h.seed(seedInput{Codigo: "EXCEL-EX-" + sufijo, Nombres: "Original", Apellidos: "Previo",
		ApellidoPaterno: "Previo", NumeroDocumento: dniExistente, Nivel: "PRIMARIA"})

	var versionAntes, auditAntes, matriculasAntes, familiaresAntes, personasAntes int
	contar := func(sql string, args ...any) int {
		var n int
		if err := h.admin.QueryRow(h.ctx, sql, args...).Scan(&n); err != nil {
			t.Fatalf("contar: %v", err)
		}
		return n
	}
	versionAntes = contar("SELECT version FROM estudiantes WHERE id = $1", existente)
	auditAntes = contar("SELECT count(*) FROM auditoria_estudiantes WHERE estudiante_id = $1", existente)
	matriculasAntes = contar("SELECT count(*) FROM matriculas")
	familiaresAntes = contar("SELECT count(*) FROM estudiante_familiares")
	personasAntes = contar("SELECT count(*) FROM personas")

	archivo := xlsx(t,
		[]any{"1", "DNI", dniNuevo, "Núñez", "Peña", "José Ángel", "ESTUDIANTE", "ACTIVO", "PRIMARIA"},
		[]any{"2", "CE", ce, "Quispe", "", "María", "ESTUDIANTE", "INACTIVO", "SECUNDARIA"},
		[]any{},
		[]any{"4", "", "", "Apellidoprueba", "", nombreSD, "ESTUDIANTE", "ACTIVO", "INICIAL"},
		[]any{"5", "DNI", dniExistente, "Otro", "Apellido", "Cambiado", "ESTUDIANTE", "INACTIVO", "SECUNDARIA"},
		[]any{"6", "DNI", "1" + sufijo, "Rojas", "", "Ana", "DOCENTE", "ACTIVO", "PRIMARIA"},
		[]any{"7", "DNI", dniNuevo, "Repetido", "", "Ana", "ESTUDIANTE", "ACTIVO", "PRIMARIA"},
	)
	req := &estudiantes.ImportarEstudiantesRequest{NombreArchivo: "estudiantes.xlsx", Contenido: archivo}

	t.Run("Permisos", func(t *testing.T) {
		_, err := h.client.PreviewImportacionEstudiantes(h.as(nadie), req)
		wantCode(t, err, codes.PermissionDenied, "preview sin permiso")
		_, err = h.client.PreviewImportacionEstudiantes(h.as(lector), req)
		wantCode(t, err, codes.PermissionDenied, "preview solo con view")
		_, err = h.client.ConfirmarImportacionEstudiantes(h.as(lector), &estudiantes.ImportarEstudiantesRequest{Contenido: archivo, Huella: excel.Huella(archivo)})
		wantCode(t, err, codes.PermissionDenied, "confirmar solo con view")
		_, err = h.client.PlantillaEstudiantes(h.as(lector), &estudiantes.PlantillaEstudiantesRequest{})
		wantCode(t, err, codes.PermissionDenied, "plantilla solo con view")
		_, err = h.client.ExportarEstudiantes(h.as(nadie), &estudiantes.ExportarEstudiantesRequest{})
		wantCode(t, err, codes.PermissionDenied, "exportar sin permiso")
		h.auth.failing = true
		_, err = h.client.ExportarEstudiantes(h.as(lector), &estudiantes.ExportarEstudiantesRequest{})
		h.auth.failing = false
		wantCode(t, err, codes.Unavailable, "exportar con Auth caído (fail-closed)")
	})

	var huella string
	t.Run("VistaPrevia", func(t *testing.T) {
		resp, err := h.client.PreviewImportacionEstudiantes(h.as(importador), req)
		if err != nil {
			t.Fatalf("preview: %v", err)
		}
		huella = resp.Huella
		r := resp.Resumen
		if r.Total != 6 || r.Validas != 3 || r.ConErrores != 1 || r.Duplicadas != 1 || r.YaExisten != 1 {
			t.Fatalf("resumen inesperado: %+v", r)
		}
		porFila := map[int32]*estudiantes.FilaImportacion{}
		for _, f := range resp.Filas {
			porFila[f.Fila] = f
		}
		if porFila[6].Resultado != "YA_EXISTE" || porFila[7].Resultado != "ERROR" || porFila[8].Resultado != "DUPLICADA" {
			t.Fatalf("clasificación por fila (números reales de Excel): %+v %+v %+v", porFila[6], porFila[7], porFila[8])
		}
		if !strings.Contains(strings.Join(porFila[8].Errores, " "), "fila 2") {
			t.Fatalf("el duplicado debe señalar la fila original: %v", porFila[8].Errores)
		}
		if contar("SELECT count(*) FROM estudiantes WHERE numero_documento = ANY($1)", []string{dniNuevo, ce}) != 0 {
			t.Fatal("la vista previa no debe escribir nada")
		}
	})

	t.Run("ConfirmarConOtroArchivoSeRechaza", func(t *testing.T) {
		_, err := h.client.ConfirmarImportacionEstudiantes(h.as(importador), &estudiantes.ImportarEstudiantesRequest{Contenido: archivo, Huella: strings.Repeat("0", 64)})
		wantCode(t, err, codes.FailedPrecondition, "huella distinta")
		_, err = h.client.ConfirmarImportacionEstudiantes(h.as(importador), &estudiantes.ImportarEstudiantesRequest{Contenido: archivo})
		wantCode(t, err, codes.InvalidArgument, "sin vista previa")
	})

	t.Run("DosConfirmacionesSimultaneas", func(t *testing.T) {
		var wg sync.WaitGroup
		resultados := make([]*estudiantes.ImportarEstudiantesResponse, 2)
		errs := make([]error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				resultados[i], errs[i] = h.client.ConfirmarImportacionEstudiantes(h.as(importador),
					&estudiantes.ImportarEstudiantesRequest{NombreArchivo: "estudiantes.xlsx", Contenido: archivo, Huella: huella})
			}(i)
		}
		wg.Wait()
		insertadas := 0
		for i := range resultados {
			if errs[i] != nil {
				t.Fatalf("confirmación %d: %v", i, errs[i])
			}
			insertadas += int(resultados[i].Resumen.Insertadas)
			if resultados[i].Resumen.Rechazadas != 2 {
				t.Fatalf("las filas con error y duplicadas se rechazan siempre: %+v", resultados[i].Resumen)
			}
		}
		if insertadas != 3 {
			t.Fatalf("entre ambas confirmaciones se insertan exactamente 3, no %d", insertadas)
		}
		registrarCreados(h, dniNuevo, ce, nombreSD)
		if n := contar("SELECT count(*) FROM estudiantes WHERE (tipo_documento, numero_documento) IN (('DNI', $1), ('CE', $2)) OR nombres = $3",
			dniNuevo, ce, nombreSD); n != 3 {
			t.Fatalf("sin duplicados en el maestro: %d registros", n)
		}
	})

	t.Run("ReintentoEsIdempotente", func(t *testing.T) {
		resp, err := h.client.ConfirmarImportacionEstudiantes(h.as(importador), &estudiantes.ImportarEstudiantesRequest{Contenido: archivo, Huella: huella})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Resumen.Insertadas != 0 || resp.Resumen.Omitidas != 4 {
			t.Fatalf("reintentar no crea nada (incluida la fila sin documento): %+v", resp.Resumen)
		}
	})

	t.Run("DatosGuardadosYAuditados", func(t *testing.T) {
		var nombres, apellidos, nivel, estado, codigo string
		if err := h.admin.QueryRow(h.ctx, "SELECT nombres, apellidos, nivel, estado, codigo FROM estudiantes WHERE tipo_documento = 'DNI' AND numero_documento = $1",
			dniNuevo).Scan(&nombres, &apellidos, &nivel, &estado, &codigo); err != nil {
			t.Fatal(err)
		}
		if nombres != "José Ángel" || apellidos != "Núñez Peña" || nivel != "PRIMARIA" || estado != "ACTIVO" || codigo != "DNI"+dniNuevo {
			t.Fatalf("datos importados (tildes, cero inicial, nivel, código): %s|%s|%s|%s|%s", nombres, apellidos, nivel, estado, codigo)
		}
		if n := contar(`SELECT count(*) FROM auditoria_estudiantes a JOIN estudiantes e ON e.id = a.estudiante_id
			WHERE a.accion = 'CREATE' AND a.usuario_id = $1 AND (e.numero_documento = ANY($2) OR e.nombres = $3)`,
			importador, []string{dniNuevo, ce}, nombreSD); n != 3 {
			t.Fatalf("cada alta queda auditada (CREATE, usuario): %d", n)
		}
	})

	t.Run("ExistenteYOtrasTablasIntactas", func(t *testing.T) {
		var nombres, nivel, estado string
		if err := h.admin.QueryRow(h.ctx, "SELECT nombres, nivel, estado FROM estudiantes WHERE id = $1", existente).Scan(&nombres, &nivel, &estado); err != nil {
			t.Fatal(err)
		}
		if nombres != "Original" || nivel != "PRIMARIA" || estado != "ACTIVO" ||
			contar("SELECT version FROM estudiantes WHERE id = $1", existente) != versionAntes ||
			contar("SELECT count(*) FROM auditoria_estudiantes WHERE estudiante_id = $1", existente) != auditAntes {
			t.Fatalf("el estudiante existente no debe modificarse: %s %s %s", nombres, nivel, estado)
		}
		if contar("SELECT count(*) FROM matriculas") != matriculasAntes ||
			contar("SELECT count(*) FROM estudiante_familiares") != familiaresAntes ||
			contar("SELECT count(*) FROM personas") != personasAntes {
			t.Fatal("la importación no debe tocar matrículas, familia ni personas")
		}
	})

	t.Run("SinDocumentoConMismoNombreSoloAvisa", func(t *testing.T) {
		otro := xlsx(t, []any{"1", "SIN_DOCUMENTO", "", "Apellidoprueba", "", nombreSD, "ESTUDIANTE", "ACTIVO", "INICIAL"})
		resp, err := h.client.PreviewImportacionEstudiantes(h.as(importador), &estudiantes.ImportarEstudiantesRequest{Contenido: otro})
		if err != nil {
			t.Fatal(err)
		}
		f := resp.Filas[0]
		if f.Resultado != "VALIDA" || !strings.Contains(strings.Join(f.Avisos, " "), "mismo nombre") {
			t.Fatalf("otro archivo con el mismo nombre sin documento: válida con aviso, got %+v", f)
		}
	})

	t.Run("Exportar", func(t *testing.T) {
		resp, err := h.client.ExportarEstudiantes(h.as(lector), &estudiantes.ExportarEstudiantesRequest{Nivel: "PRIMARIA", Estado: "ACTIVO"})
		if err != nil {
			t.Fatalf("exportar: %v", err)
		}
		if !strings.HasPrefix(resp.Nombre, "estudiantes_") || !strings.HasSuffix(resp.Nombre, ".xlsx") || resp.Filas < 2 {
			t.Fatalf("nombre/filas: %s %d", resp.Nombre, resp.Filas)
		}
		f, err := excelize.OpenReader(bytes.NewReader(resp.Contenido))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
		rows, _ := f.GetRows(excel.HojaEstudiantes)
		if strings.Join(rows[0], "|") != strings.Join(excel.Encabezados, "|") {
			t.Fatalf("encabezados exportados: %v", rows[0])
		}
		var encontrado bool
		for _, r := range rows[1:] {
			if len(r) > 9 {
				t.Fatalf("más de 9 columnas: %v", r)
			}
			if len(r) > 2 && r[2] == dniNuevo {
				encontrado = r[1] == "DNI" && r[3] == "Núñez" && r[6] == excel.Condicion && r[8] == "PRIMARIA"
			}
			if len(r) > 2 && r[2] == ce {
				t.Fatal("el filtro (PRIMARIA, ACTIVO) no debe incluir al CE de SECUNDARIA/INACTIVO")
			}
		}
		if !encontrado {
			t.Fatal("la exportación debe reflejar el estudiante importado, con el DNI como texto")
		}

		_, err = h.client.ExportarEstudiantes(h.as(lector), &estudiantes.ExportarEstudiantesRequest{Nivel: "UNIVERSITARIO"})
		wantCode(t, err, codes.InvalidArgument, "nivel no admitido")
		h.svcCtx.Config.Excel.MaxFilasExportacion = 1
		_, err = h.client.ExportarEstudiantes(h.as(lector), &estudiantes.ExportarEstudiantesRequest{})
		h.svcCtx.Config.Excel.MaxFilasExportacion = 0
		wantCode(t, err, codes.FailedPrecondition, "supera el máximo de exportación")
	})
}

// registrarCreados agrega a la limpieza los estudiantes que creó la importación.
func registrarCreados(h *harness, dni, ce, nombreSD string) {
	rows, err := h.admin.Query(h.ctx, `SELECT id FROM estudiantes
		WHERE (tipo_documento = 'DNI' AND numero_documento = $1) OR (tipo_documento = 'CE' AND numero_documento = $2) OR nombres = $3`, dni, ce, nombreSD)
	if err != nil {
		h.t.Fatalf("buscar creados: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			h.created = append(h.created, id)
		}
	}
}

// xlsx arma un archivo con los encabezados oficiales y las filas dadas.
func xlsx(t *testing.T, filas ...[]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	_ = f.SetSheetName("Sheet1", excel.HojaEstudiantes)
	cab := make([]any, len(excel.Encabezados))
	for i, h := range excel.Encabezados {
		cab[i] = h
	}
	for i, fila := range append([][]any{cab}, filas...) {
		celda, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(excel.HojaEstudiantes, celda, &fila); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
