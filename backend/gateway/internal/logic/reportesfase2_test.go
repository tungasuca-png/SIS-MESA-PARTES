package logic

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"derivaciones/derivacionesclient"
	"documentos/documentosclient"
	"expedientes/expedientesclient"
	"gateway/internal/svc"
	"gateway/internal/types"
	"usuarios/usuariosclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Dobles: solo los métodos de reporte. Datos ficticios (los "nombres" son
// inventados para comprobar que nunca salen en la respuesta).

type expFase2 struct {
	expedientesclient.Expedientes
	pedidos []*expedientesclient.ReporteExpedientesRequest
	totales map[string]int32 // desde -> total
}

func (e *expFase2) ReporteExpedientes(_ context.Context, in *expedientesclient.ReporteExpedientesRequest, _ ...grpc.CallOption) (*expedientesclient.ReporteExpedientesResponse, error) {
	e.pedidos = append(e.pedidos, in)
	total := e.totales[in.Desde]
	r := &expedientesclient.ReporteExpedientesResponse{
		Total: total, Pendientes: total / 2,
		PorEstado: []*expedientesclient.ReporteConteo{{Clave: "PENDIENTE", Cantidad: total / 2}, {Clave: "ATENDIDO", Cantidad: total - total/2}},
		Alcance:   "INSTITUCION", Generado: "2026-10-30T09:15:00-05:00",
		Serie:        []*expedientesclient.ReporteConteo{{Clave: "2026-10-26", Cantidad: total}},
		Granularidad: in.Granularidad,
		AreaEstado:   []*expedientesclient.ReporteCruce{{Fila: "SECRETARIA", Columna: "PENDIENTE", Cantidad: total / 2}},
	}
	if in.IncluirRegistros {
		r.RegistrosPorUsuario = []*expedientesclient.ReporteConteo{{Clave: "AAAA-SEC", Cantidad: 4}, {Clave: "bbbb-sol", Cantidad: 2}}
	}
	return r, nil
}

type derFase2 struct {
	derivacionesclient.Derivaciones
	pedido *derivacionesclient.ReporteDerivacionesRequest
}

func (d *derFase2) ReporteDerivaciones(_ context.Context, in *derivacionesclient.ReporteDerivacionesRequest, _ ...grpc.CallOption) (*derivacionesclient.ReporteDerivacionesResponse, error) {
	d.pedido = in
	r := &derivacionesclient.ReporteDerivacionesResponse{
		Total: 3, SinClasificar: 1, Granularidad: "MES", Alcance: "INSTITUCION", Generado: "2026-10-30T09:15:00-05:00",
		PorDestino: []*derivacionesclient.ReporteDerivConteo{{Clave: "DIRECTOR", Cantidad: 2}, {Clave: "SIN_CLASIFICAR", Cantidad: 1}},
		Flujo:      []*derivacionesclient.ReporteDerivFlujo{{Origen: "SECRETARIA", Destino: "DIRECTOR", Cantidad: 2}},
	}
	if in.IncluirRegistros {
		r.RegistrosPorUsuario = []*derivacionesclient.ReporteDerivConteo{{Clave: "aaaa-sec", Cantidad: 1}, {Clave: "cccc-dir", Cantidad: 3}}
		// Solo tipo DERIVACION (subconjunto de los movimientos).
		r.DerivacionesPorUsuario = []*derivacionesclient.ReporteDerivConteo{{Clave: "AAAA-SEC", Cantidad: 1}, {Clave: "cccc-dir", Cantidad: 2}}
	}
	return r, nil
}

type docFase2 struct {
	documentosclient.Documentos
	pedido *documentosclient.ReporteDocumentosRequest
}

func (d *docFase2) ReporteDocumentos(_ context.Context, in *documentosclient.ReporteDocumentosRequest, _ ...grpc.CallOption) (*documentosclient.ReporteDocumentosResponse, error) {
	d.pedido = in
	r := &documentosclient.ReporteDocumentosResponse{Total: 2, Granularidad: "MES", Alcance: "INSTITUCION", Generado: "2026-10-30T09:15:00-05:00",
		PorTipoDocumento: []*documentosclient.ReporteDocConteo{{Clave: "ADJUNTO", Cantidad: 2}}}
	if in.IncluirRegistros {
		r.RegistrosPorUsuario = []*documentosclient.ReporteDocConteo{{Clave: "cccc-dir", Cantidad: 2}}
	}
	return r, nil
}

type usuFase2 struct {
	usuariosclient.Usuarios
	pedidos [][]string
}

func (u *usuFase2) GetUsuariosBasic(_ context.Context, in *usuariosclient.GetUsuariosBasicRequest, _ ...grpc.CallOption) (*usuariosclient.GetUsuariosBasicResponse, error) {
	u.pedidos = append(u.pedidos, in.Ids)
	datos := map[string][3]string{
		"aaaa-sec": {"SECRETARIA", "Rosa Quispe Ficticia", "11111111"},
		"bbbb-sol": {"SOLICITANTE", "Pedro Mamani Ficticio", "22222222"},
		"cccc-dir": {"DIRECTOR", "Carlos Huamán Ficticio", "33333333"},
	}
	out := &usuariosclient.GetUsuariosBasicResponse{}
	for _, id := range in.Ids {
		if d, ok := datos[id]; ok {
			out.Usuarios = append(out.Usuarios, &usuariosclient.UsuarioBasic{Id: id, TipoUsuario: d[0], NombreCompleto: d[1], Dni: d[2]})
		}
	}
	return out, nil
}

func ctxFase2() (*svc.ServiceContext, *expFase2, *derFase2, *docFase2, *usuFase2) {
	e := &expFase2{totales: map[string]int32{"2026-10-01": 12, "2026-09-01": 10, "2025-10-01": 4}}
	d, doc, u := &derFase2{}, &docFase2{}, &usuFase2{}
	return &svc.ServiceContext{ExpedientesClient: e, DerivacionesClient: d, DocumentosClient: doc, UsuariosClient: u}, e, d, doc, u
}

func TestReporteExpedientes_Fase2MapeaSerieYCruces(t *testing.T) {
	sc, e, _, _, _ := ctxFase2()
	r, err := NewReporteExpedientesLogic(context.Background(), sc).ReporteExpedientes(&types.ReporteExpedientesRequest{Desde: "2026-10-01", Granularidad: "SEMANA"})
	if err != nil {
		t.Fatal(err)
	}
	if e.pedidos[0].Granularidad != "SEMANA" || len(r.Serie) != 1 || len(r.AreaEstado) != 1 || r.Granularidad != "SEMANA" {
		t.Fatalf("serie y cruces: %+v", r)
	}
	if e.pedidos[0].IncluirRegistros {
		t.Fatal("el endpoint público nunca pide registros por usuario")
	}
}

func TestComparacion_DosConsultasYUmbral(t *testing.T) {
	sc, e, _, _, _ := ctxFase2()
	l := NewReporteComparacionLogic(context.Background(), sc)
	r, err := l.ReporteComparacion(&types.ReporteComparacionRequest{Desde: "2026-10-01", Hasta: "2026-10-30", Modo: "PERSONALIZADO", DesdeB: "2026-09-01", HastaB: "2026-09-30", Tipo: "PERMISO"})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.pedidos) != 2 || e.pedidos[0].Tipo != "PERMISO" || e.pedidos[1].Tipo != "PERMISO" || e.pedidos[1].Desde != "2026-09-01" {
		t.Fatalf("ambos periodos con los mismos filtros: %+v", e.pedidos)
	}
	if !r.Comparable || r.TotalA != 12 || r.TotalB != 10 || r.Minimo != 10 {
		t.Fatalf("12 frente a 10: comparable: %+v", r)
	}
	if r.Filas[0].Clave != "TOTAL" || !r.Filas[0].TieneVariacion || r.Filas[0].Variacion != 20 {
		t.Fatalf("variación del total: %+v", r.Filas[0])
	}

	// Año anterior con 4 expedientes: datos insuficientes, sin variaciones.
	sc2, _, _, _, _ := ctxFase2()
	r2, err := NewReporteComparacionLogic(context.Background(), sc2).ReporteComparacion(&types.ReporteComparacionRequest{Desde: "2026-10-01", Hasta: "2026-10-30", Modo: "ANIO_ANTERIOR"})
	if err != nil || r2.Comparable || r2.Mensaje == "" || r2.DesdeB != "2025-10-01" {
		t.Fatalf("insuficiente: %+v %v", r2, err)
	}
	for _, f := range r2.Filas {
		if f.TieneVariacion {
			t.Fatal("sin variaciones con datos insuficientes")
		}
	}
	// Sin fechas: 400 sin consultar.
	sc3, e3, _, _, _ := ctxFase2()
	if _, err := NewReporteComparacionLogic(context.Background(), sc3).ReporteComparacion(&types.ReporteComparacionRequest{Desde: "2026-10-01"}); status.Code(err) != codes.InvalidArgument || len(e3.pedidos) != 0 {
		t.Fatalf("sin hasta: %v", err)
	}
}

func TestActividad_SoloRolesSinIdentidades(t *testing.T) {
	sc, e, d, doc, u := ctxFase2()
	r, err := NewReporteActividadLogic(context.Background(), sc).ReporteActividad(&types.ReportePeriodoRequest{Desde: "2026-10-01", Tipo: "CONSTANCIA"})
	if err != nil {
		t.Fatal(err)
	}
	if !e.pedidos[0].IncluirRegistros || !d.pedido.IncluirRegistros || !doc.pedido.IncluirRegistros || d.pedido.TipoTramite != "CONSTANCIA" {
		t.Fatal("debe pedir los registros a los tres servicios con los mismos filtros")
	}
	if len(u.pedidos) != 1 || len(u.pedidos[0]) != 3 {
		t.Fatalf("una consulta a Usuarios con IDs únicos: %v", u.pedidos)
	}
	por := map[string]types.ActividadRolDTO{}
	for _, x := range r.Roles {
		por[x.Rol] = x
	}
	if s := por["SECRETARIA"]; s.ExpedientesRegistrados != 4 || s.MovimientosRegistrados != 1 || s.DerivacionesRegistradas != 1 || s.Total != 5 {
		t.Fatalf("secretaría (IDs con distinta capitalización se unifican): %+v", s)
	}
	// Total = expedientes + movimientos + documentos (las derivaciones ya
	// están dentro de los movimientos).
	if dir := por["DIRECTOR"]; dir.MovimientosRegistrados != 3 || dir.DerivacionesRegistradas != 2 || dir.DocumentosSubidos != 2 || dir.Total != 5 {
		t.Fatalf("dirección: %+v", dir)
	}
	if por["SOLICITANTE"].ExpedientesRegistrados != 2 {
		t.Fatal("solicitantes externos")
	}
	j, _ := json.Marshal(r)
	for _, prohibido := range []string{"aaaa", "bbbb", "cccc", "Rosa", "Pedro", "Carlos", "11111111", "22222222"} {
		if strings.Contains(string(j), prohibido) {
			t.Fatalf("la respuesta expone %q", prohibido)
		}
	}
}

func decodificar(t *testing.T, r *types.ArchivoReporteResponse) string {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(r.ContenidoBase64)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestExportaciones_PorPestana(t *testing.T) {
	sc, e, d, _, _ := ctxFase2()
	ctx := context.Background()

	resumen, err := NewExportarReporteExpedientesLogic(ctx, sc).ExportarReporteExpedientes(&types.ExportarReporteExpedientesRequest{Formato: "csv", Desde: "2026-10-01"})
	if err != nil || !strings.HasPrefix(resumen.Nombre, "reporte-expedientes-2026") || !strings.Contains(decodificar(t, resumen), "Reporte de expedientes – Mesa de Partes") {
		t.Fatalf("vista por defecto = Resumen de la Fase 1: %v", err)
	}
	if e.pedidos[0].Granularidad != "" {
		t.Fatal("el Resumen no pide granularidad (consulta de la Fase 1)")
	}
	areas, err := NewExportarReporteExpedientesLogic(ctx, sc).ExportarReporteExpedientes(&types.ExportarReporteExpedientesRequest{Formato: "csv", Vista: "expedientes", Desde: "2026-10-01", Granularidad: "SEMANA"})
	if err != nil || !strings.HasPrefix(areas.Nombre, "reporte-expedientes-areas-") || !strings.Contains(decodificar(t, areas), "Área actual × estado") {
		t.Fatalf("vista expedientes: %v", err)
	}
	if _, err := NewExportarReporteExpedientesLogic(ctx, sc).ExportarReporteExpedientes(&types.ExportarReporteExpedientesRequest{Formato: "csv", Vista: "otra"}); status.Code(err) != codes.InvalidArgument {
		t.Fatal("vista inválida")
	}
	der, err := NewExportarReporteDerivacionesLogic(ctx, sc).ExportarReporteDerivaciones(&types.ExportarReportePeriodoRequest{Formato: "csv", Desde: "2026-10-01", Tipo: "PERMISO", Granularidad: "MES"})
	if err != nil || d.pedido.TipoTramite != "PERMISO" || d.pedido.Desde != "2026-10-01" || d.pedido.IncluirRegistros {
		t.Fatalf("derivaciones con los filtros aplicados y sin IDs: %v %+v", err, d.pedido)
	}
	if s := decodificar(t, der); !strings.Contains(s, "Filtro tipo de trámite;Permiso") || !strings.Contains(s, "Sin clasificar") {
		t.Fatalf("csv derivaciones:\n%s", s)
	}
	doc, err := NewExportarReporteDocumentosLogic(ctx, sc).ExportarReporteDocumentos(&types.ExportarReportePeriodoRequest{Formato: "xlsx"})
	if err != nil || !bytes.HasPrefix([]byte(decodificar(t, doc)), []byte("PK")) {
		t.Fatalf("xlsx documentos: %v", err)
	}
	act, err := NewExportarReporteActividadLogic(ctx, sc).ExportarReporteActividad(&types.ExportarReportePeriodoRequest{Formato: "csv"})
	if err != nil || strings.Contains(decodificar(t, act), "Rosa") || !strings.Contains(decodificar(t, act), "Secretaría") {
		t.Fatalf("actividad: %v", err)
	}
	comp, err := NewExportarReporteComparacionLogic(ctx, sc).ExportarReporteComparacion(&types.ExportarReporteComparacionRequest{Formato: "csv", Desde: "2026-10-01", Hasta: "2026-10-30", Modo: "ANIO_ANTERIOR"})
	if err != nil || !strings.Contains(decodificar(t, comp), "Datos insuficientes para comparar") {
		t.Fatalf("comparación: %v", err)
	}
	for _, f := range []func() error{
		func() error {
			_, err := NewExportarReporteDerivacionesLogic(ctx, sc).ExportarReporteDerivaciones(&types.ExportarReportePeriodoRequest{Formato: "doc"})
			return err
		},
		func() error {
			_, err := NewExportarReporteComparacionLogic(ctx, sc).ExportarReporteComparacion(&types.ExportarReporteComparacionRequest{Formato: "html", Desde: "2026-10-01", Hasta: "2026-10-30"})
			return err
		},
	} {
		if status.Code(f()) != codes.InvalidArgument {
			t.Fatal("formato no admitido")
		}
	}
	// PDF en cada pestaña, con su nombre y tipo.
	pdfs := map[string]func() (*types.ArchivoReporteResponse, error){
		"expedientes": func() (*types.ArchivoReporteResponse, error) {
			return NewExportarReporteExpedientesLogic(ctx, sc).ExportarReporteExpedientes(&types.ExportarReporteExpedientesRequest{Formato: "pdf"})
		},
		"expedientes-areas": func() (*types.ArchivoReporteResponse, error) {
			return NewExportarReporteExpedientesLogic(ctx, sc).ExportarReporteExpedientes(&types.ExportarReporteExpedientesRequest{Formato: "pdf", Vista: "EXPEDIENTES", Granularidad: "MES"})
		},
		"derivaciones": func() (*types.ArchivoReporteResponse, error) {
			return NewExportarReporteDerivacionesLogic(ctx, sc).ExportarReporteDerivaciones(&types.ExportarReportePeriodoRequest{Formato: "pdf"})
		},
		"documentos": func() (*types.ArchivoReporteResponse, error) {
			return NewExportarReporteDocumentosLogic(ctx, sc).ExportarReporteDocumentos(&types.ExportarReportePeriodoRequest{Formato: "pdf"})
		},
		"actividad": func() (*types.ArchivoReporteResponse, error) {
			return NewExportarReporteActividadLogic(ctx, sc).ExportarReporteActividad(&types.ExportarReportePeriodoRequest{Formato: "pdf"})
		},
		"comparacion": func() (*types.ArchivoReporteResponse, error) {
			return NewExportarReporteComparacionLogic(ctx, sc).ExportarReporteComparacion(&types.ExportarReporteComparacionRequest{Formato: " PDF ", Desde: "2026-10-01", Hasta: "2026-10-30"})
		},
	}
	for tipo, f := range pdfs {
		a, err := f()
		if err != nil || a.Mime != "application/pdf" || !strings.HasPrefix(a.Nombre, "reporte-"+tipo+"-") || !strings.HasSuffix(a.Nombre, ".pdf") ||
			!strings.HasPrefix(decodificar(t, a), "%PDF-") {
			t.Fatalf("PDF %s: %v %+v", tipo, err, a)
		}
	}
}
