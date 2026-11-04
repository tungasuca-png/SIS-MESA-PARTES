package logic

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"expedientes/expedientesclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// expedientesReporteFalso implementa solo ReporteExpedientes (cualquier otro
// método haría panic). Datos ficticios agregados: sin expedientes ni personas.
type expedientesReporteFalso struct {
	expedientesclient.Expedientes
	pedido *expedientesclient.ReporteExpedientesRequest
	authz  []string
	err    error
}

func (e *expedientesReporteFalso) ReporteExpedientes(ctx context.Context, in *expedientesclient.ReporteExpedientesRequest, _ ...grpc.CallOption) (*expedientesclient.ReporteExpedientesResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	e.authz = md.Get("authorization")
	e.pedido = in
	if e.err != nil {
		return nil, e.err
	}
	c := func(clave string, n int32) *expedientesclient.ReporteConteo {
		return &expedientesclient.ReporteConteo{Clave: clave, Cantidad: n}
	}
	return &expedientesclient.ReporteExpedientesResponse{
		Total:                   5,
		PorEstado:               []*expedientesclient.ReporteConteo{c("PENDIENTE", 2), c("EN_PROCESO", 1), c("OBSERVADO", 0), c("ATENDIDO", 2)},
		PorTipo:                 []*expedientesclient.ReporteConteo{c("CONSTANCIA", 5)},
		PorPrioridad:            []*expedientesclient.ReporteConteo{c("NORMAL", 4), c("URGENTE", 1)},
		PorArea:                 []*expedientesclient.ReporteConteo{c("SECRETARIA", 5)},
		PorMes:                  []*expedientesclient.ReporteConteo{c("2026-10", 5)},
		Pendientes:              3,
		PendientesPorAntiguedad: []*expedientesclient.ReporteConteo{c("0-3", 3)},
		Alcance:                 "INSTITUCION",
		Generado:                "2026-10-30T09:15:00-05:00",
	}, nil
}

func TestReporteExpedientes_TransmiteFiltrosYMapea(t *testing.T) {
	falso := &expedientesReporteFalso{}
	l := NewReporteExpedientesLogic(context.Background(), &svc.ServiceContext{ExpedientesClient: falso})
	resp, err := l.ReporteExpedientes(&types.ReporteExpedientesRequest{
		Authorization: "Bearer jwt.ficticio.x", Desde: "2026-01-01", Hasta: "2026-10-30",
		Estado: "PENDIENTE", Tipo: "CONSTANCIA", Prioridad: "URGENTE", Area: "SECRETARIA",
	})
	if err != nil {
		t.Fatal(err)
	}
	p := falso.pedido
	if p.Desde != "2026-01-01" || p.Hasta != "2026-10-30" || p.Estado != "PENDIENTE" || p.Tipo != "CONSTANCIA" || p.Prioridad != "URGENTE" || p.Area != "SECRETARIA" {
		t.Fatalf("filtros no transmitidos: %+v", p)
	}
	if len(falso.authz) != 1 || falso.authz[0] != "Bearer jwt.ficticio.x" {
		t.Fatal("el JWT debe reenviarse: Expedientes valida reportes.view")
	}
	if resp.Total != 5 || resp.Pendientes != 3 || len(resp.PorEstado) != 4 || resp.PorMes[0].Clave != "2026-10" || resp.Alcance != "INSTITUCION" {
		t.Fatalf("mapeo: %+v", resp)
	}
	j, _ := json.Marshal(resp)
	for _, campo := range []string{`"por_estado"`, `"por_tipo"`, `"por_prioridad"`, `"por_area"`, `"por_mes"`, `"pendientes_por_antiguedad"`, `"generado"`} {
		if !strings.Contains(string(j), campo) {
			t.Errorf("falta %s en el JSON", campo)
		}
	}
}

func TestReporteExpedientes_PropagaErrores(t *testing.T) {
	for _, c := range []codes.Code{codes.PermissionDenied, codes.InvalidArgument, codes.Unauthenticated} {
		falso := &expedientesReporteFalso{err: status.Error(c, "x")}
		l := NewReporteExpedientesLogic(context.Background(), &svc.ServiceContext{ExpedientesClient: falso})
		if _, err := l.ReporteExpedientes(&types.ReporteExpedientesRequest{}); status.Code(err) != c {
			t.Errorf("se esperaba %v: %v", c, err)
		}
		le := NewExportarReporteExpedientesLogic(context.Background(), &svc.ServiceContext{ExpedientesClient: falso})
		if _, err := le.ExportarReporteExpedientes(&types.ExportarReporteExpedientesRequest{Formato: "csv"}); status.Code(err) != c {
			t.Errorf("exportar: se esperaba %v: %v", c, err)
		}
	}
}

func TestExportarReporte_MismosFiltrosYFormatos(t *testing.T) {
	falso := &expedientesReporteFalso{}
	l := NewExportarReporteExpedientesLogic(context.Background(), &svc.ServiceContext{ExpedientesClient: falso})

	if _, err := l.ExportarReporteExpedientes(&types.ExportarReporteExpedientesRequest{Formato: "doc"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("formato no admitido: %v", err)
	}
	if falso.pedido != nil {
		t.Fatal("con formato inválido no debe consultarse el reporte")
	}

	csv, err := l.ExportarReporteExpedientes(&types.ExportarReporteExpedientesRequest{
		Authorization: "Bearer jwt.ficticio.x", Formato: " CSV ", Desde: "2026-01-01", Hasta: "2026-10-30", Tipo: "constancia",
	})
	if err != nil {
		t.Fatal(err)
	}
	if falso.pedido.Desde != "2026-01-01" || falso.pedido.Tipo != "constancia" || falso.authz[0] != "Bearer jwt.ficticio.x" {
		t.Fatalf("la exportación debe usar exactamente los mismos filtros: %+v", falso.pedido)
	}
	if csv.Nombre != "reporte-expedientes-20261030-0915.csv" || csv.Mime != "text/csv" {
		t.Fatalf("archivo: %s %s", csv.Nombre, csv.Mime)
	}
	contenido, _ := base64.StdEncoding.DecodeString(csv.ContenidoBase64)
	for _, esperado := range []string{"Del 01/01/2026 al 30/10/2026", "Constancia de Estudios", "Total de expedientes;5"} {
		if !bytes.Contains(contenido, []byte(esperado)) {
			t.Errorf("falta %q en el CSV exportado", esperado)
		}
	}

	xlsx, err := l.ExportarReporteExpedientes(&types.ExportarReporteExpedientesRequest{Formato: "xlsx"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := base64.StdEncoding.DecodeString(xlsx.ContenidoBase64)
	if !strings.HasSuffix(xlsx.Nombre, ".xlsx") || !bytes.HasPrefix(b, []byte("PK")) {
		t.Fatal("el XLSX debe ser un ZIP válido")
	}
}
