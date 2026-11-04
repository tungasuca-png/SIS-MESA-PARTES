package logic

import (
	"context"
	"strings"
	"testing"
	"time"

	"expedientes/expedientes"
	"expedientes/internal/estados"
	"expedientes/internal/interceptor"
	"expedientes/internal/repository"
	"expedientes/internal/security"
	"expedientes/internal/svc"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Pruebas del reporte sin PostgreSQL: validación, permisos (antes de tocar la
// base: el repositorio queda nil) y armado de agrupaciones con filas fijas.

var hoyPrueba = time.Date(2026, 10, 30, 9, 0, 0, 0, zonaReportes)

func reporteComo(t *testing.T, auth *authFalso, role string, in *expedientes.ReporteExpedientesRequest) error {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, security.Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "00000000-0000-4000-8000-000000000001",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}).SignedString([]byte(secretoListado))
	if err != nil {
		t.Fatal(err)
	}
	sc := &svc.ServiceContext{AuthClient: auth}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tok))
	_, err = interceptor.AuthenticationInterceptor(security.NewJWTValidator(secretoListado))(ctx, in, &grpc.UnaryServerInfo{},
		func(ctx context.Context, req any) (any, error) {
			return NewReporteExpedientesLogic(ctx, sc).ReporteExpedientes(req.(*expedientes.ReporteExpedientesRequest))
		})
	return err
}

func TestReporte_ExigeReportesView(t *testing.T) {
	// expedientes.view no basta.
	sin := &authFalso{permitido: map[string]bool{"expedientes.view": true}}
	for _, rol := range []string{"ADMIN", "SECRETARIA", "DIRECTOR", "DOCENTE"} {
		if err := reporteComo(t, sin, rol, &expedientes.ReporteExpedientesRequest{}); status.Code(err) != codes.PermissionDenied {
			t.Errorf("%s sin reportes.view: se esperaba PermissionDenied, got %v", rol, err)
		}
	}
	// Auth caído: falla cerrado.
	if err := reporteComo(t, &authFalso{caido: true}, "ADMIN", &expedientes.ReporteExpedientesRequest{}); status.Code(err) == codes.OK {
		t.Error("con Auth caído no debe responder datos")
	}
	// Un rol externo con el permiso asignado por error sigue sin acceso.
	con := &authFalso{permitido: map[string]bool{"reportes.view": true}}
	if err := reporteComo(t, con, "SOLICITANTE", &expedientes.ReporteExpedientesRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Errorf("SOLICITANTE: se esperaba PermissionDenied, got %v", err)
	}
}

func TestReporte_FiltrosInvalidosNoLleganASQL(t *testing.T) {
	auth := &authFalso{permitido: map[string]bool{"reportes.view": true}}
	for nombre, in := range map[string]*expedientes.ReporteExpedientesRequest{
		"estado inyectado":   {Estado: "PENDIENTE' OR 1=1--"},
		"tipo inválido":      {Tipo: "X"},
		"prioridad inválida": {Prioridad: "ALTA"},
		"área inválida":      {Area: "PSICOLOGIA"},
		"fecha mal formada":  {Desde: "30/10/2026"},
		"fecha inexistente":  {Hasta: "2026-02-30"},
		"desde > hasta":      {Desde: "2026-10-01", Hasta: "2026-09-01"},
	} {
		err := reporteComo(t, auth, "SECRETARIA", in)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: se esperaba InvalidArgument, got %v", nombre, err)
		}
	}
}

func TestValidarFiltroReporte(t *testing.T) {
	ok := func(in *expedientes.ReporteExpedientesRequest) repository.ReporteFilter {
		t.Helper()
		f, err := validarFiltroReporte(in, hoyPrueba)
		if err != nil {
			t.Fatalf("%+v: %v", in, err)
		}
		return f
	}
	f := ok(&expedientes.ReporteExpedientesRequest{Desde: " 2026-01-01 ", Hasta: "2026-10-30", Estado: "pendiente", Tipo: "constancia", Prioridad: "urgente", Area: "director"})
	if f.Desde != "2026-01-01" || f.Estado != "PENDIENTE" || f.Tipo != "CONSTANCIA" || f.Prioridad != "URGENTE" || f.Area != "DIRECTOR" || f.Hoy != "2026-10-30" {
		t.Fatalf("normalización inesperada: %+v", f)
	}
	ok(&expedientes.ReporteExpedientesRequest{})                                         // sin filtros
	ok(&expedientes.ReporteExpedientesRequest{Desde: "2026-10-30", Hasta: "2026-10-30"}) // un solo día
	ok(&expedientes.ReporteExpedientesRequest{Hasta: "2027-12-31"})                      // hasta futuro admitido

	for nombre, in := range map[string]*expedientes.ReporteExpedientesRequest{
		"desde futuro":        {Desde: "2026-10-31"},
		"desde antes de 2000": {Desde: "1999-12-31"},
		"hasta antes de 2000": {Hasta: "1999-12-31"},
		"más de 10 años":      {Desde: "2010-01-01", Hasta: "2020-01-02"},
	} {
		_, err := validarFiltroReporte(in, hoyPrueba)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: se esperaba InvalidArgument, got %v", nombre, err)
		}
		if err != nil && (strings.Contains(err.Error(), "1999-12-31") || strings.Contains(err.Error(), "2026-10-31")) {
			t.Errorf("%s: el error no debe repetir el valor recibido", nombre)
		}
	}
	if _, err := validarFiltroReporte(&expedientes.ReporteExpedientesRequest{Desde: "2010-01-01", Hasta: "2020-01-01"}, hoyPrueba); err != nil {
		t.Errorf("exactamente 10 años debe aceptarse: %v", err)
	}
}

func TestRangoAntiguedad(t *testing.T) {
	casos := map[int]string{-1: "0-3", 0: "0-3", 3: "0-3", 4: "4-7", 7: "4-7", 8: "8-15", 15: "8-15", 16: "16-30", 30: "16-30", 31: "31+", 400: "31+"}
	for dias, esperado := range casos {
		if got := rangoAntiguedad(dias); got != esperado {
			t.Errorf("%d días: %s, se esperaba %s", dias, got, esperado)
		}
	}
}

func TestMesesContinuos(t *testing.T) {
	got := mesesContinuos("2025-11", "2026-02")
	if strings.Join(got, ",") != "2025-11,2025-12,2026-01,2026-02" {
		t.Fatalf("serie inesperada: %v", got)
	}
	if len(mesesContinuos("2026-03", "2026-02")) != 0 || len(mesesContinuos("x", "2026-02")) != 0 {
		t.Fatal("rangos inválidos deben devolver vacío")
	}
}

func dias(n int) *int { return &n }

func mapa(cs []*expedientes.ReporteConteo) map[string]int32 {
	m := map[string]int32{}
	for _, c := range cs {
		m[c.Clave] = c.Cantidad
	}
	return m
}

func TestArmarReporte(t *testing.T) {
	filas := []repository.ReporteFila{
		{Estado: "PENDIENTE", Tipo: "CONSTANCIA", Prioridad: "NORMAL", Area: "SECRETARIA", Mes: "2026-08", DiasPendiente: dias(2), Cantidad: 3},
		{Estado: "EN_PROCESO", Tipo: "PERMISO", Prioridad: "URGENTE", Area: "DIRECTOR", Mes: "2026-10", DiasPendiente: dias(20), Cantidad: 2},
		{Estado: "EN_PROCESO", Tipo: "PERMISO", Prioridad: "NORMAL", Area: "DIRECTOR", Mes: "2026-06", DiasPendiente: dias(120), Cantidad: 1},
		{Estado: "ATENDIDO", Tipo: "CONSTANCIA", Prioridad: "NORMAL", Area: "SECRETARIA", Mes: "2026-08", Cantidad: 4},
		{Estado: "OBSERVADO", Tipo: "OTRO", Prioridad: "NORMAL", Area: "SECRETARIA", Mes: "2026-10", Cantidad: 1},
	}
	r := armarReporte(filas, repository.ReporteFilter{}, hoyPrueba)
	if r.Total != 11 || r.Pendientes != 6 {
		t.Fatalf("total %d / pendientes %d", r.Total, r.Pendientes)
	}
	if e := mapa(r.PorEstado); e["PENDIENTE"] != 3 || e["EN_PROCESO"] != 3 || e["ATENDIDO"] != 4 || e["OBSERVADO"] != 1 || len(r.PorEstado) != 4 {
		t.Fatalf("por estado: %v", e)
	}
	if r.PorEstado[0].Clave != "PENDIENTE" || r.PorEstado[3].Clave != "ATENDIDO" {
		t.Fatal("el orden de estados debe ser estable")
	}
	if len(r.PorTipo) != 9 || mapa(r.PorTipo)["PERMISO"] != 3 || mapa(r.PorTipo)["OFICIO"] != 0 {
		t.Fatalf("por tipo (catálogo completo con ceros): %v", mapa(r.PorTipo))
	}
	if p := mapa(r.PorPrioridad); p["URGENTE"] != 2 || p["NORMAL"] != 9 {
		t.Fatalf("por prioridad: %v", p)
	}
	if a := mapa(r.PorArea); a["DIRECTOR"] != 3 || a["SECRETARIA"] != 8 || len(r.PorArea) != 6 {
		t.Fatalf("por área: %v", a)
	}
	// Serie continua del primer dato al mes actual, con ceros.
	var meses []string
	for _, m := range r.PorMes {
		meses = append(meses, m.Clave)
	}
	if strings.Join(meses, ",") != "2026-06,2026-07,2026-08,2026-09,2026-10" || mapa(r.PorMes)["2026-07"] != 0 || mapa(r.PorMes)["2026-08"] != 7 {
		t.Fatalf("serie mensual: %v %v", meses, mapa(r.PorMes))
	}
	if a := mapa(r.PendientesPorAntiguedad); a["0-3"] != 3 || a["16-30"] != 2 || a["31+"] != 1 || a["4-7"] != 0 || len(r.PendientesPorAntiguedad) != 5 {
		t.Fatalf("antigüedad: %v", a)
	}
	// Suma de cada agrupación = total.
	for nombre, grupo := range map[string][]*expedientes.ReporteConteo{"estado": r.PorEstado, "tipo": r.PorTipo, "prioridad": r.PorPrioridad, "área": r.PorArea, "mes": r.PorMes} {
		suma := int32(0)
		for _, c := range grupo {
			suma += c.Cantidad
		}
		if suma != r.Total {
			t.Errorf("la suma por %s (%d) no cuadra con el total (%d)", nombre, suma, r.Total)
		}
	}
}

func TestArmarReporte_SerieSegunPeriodo(t *testing.T) {
	f := repository.ReporteFilter{Desde: "2026-01-15", Hasta: "2026-03-10"}
	r := armarReporte(nil, f, hoyPrueba)
	if len(r.PorMes) != 3 || r.PorMes[0].Clave != "2026-01" || r.PorMes[2].Clave != "2026-03" {
		t.Fatalf("serie del periodo: %v", mapa(r.PorMes))
	}
}

func TestArmarReporte_SinResultados(t *testing.T) {
	r := armarReporte(nil, repository.ReporteFilter{}, hoyPrueba)
	if r.Total != 0 || r.Pendientes != 0 || len(r.PorMes) != 0 {
		t.Fatalf("vacío: %+v", r)
	}
	// Los catálogos siguen presentes (en cero) para que la interfaz no cambie de forma.
	if len(r.PorEstado) != 4 || len(r.PorTipo) != 9 || len(r.PorPrioridad) != 2 || len(r.PorArea) != 6 || len(r.PendientesPorAntiguedad) != 5 {
		t.Fatal("los catálogos deben venir completos aunque no haya datos")
	}
}

func TestCatalogosDelReporteSonValidos(t *testing.T) {
	for _, e := range ordenEstados {
		if !estados.IsValidEstado(e) {
			t.Errorf("estado %s", e)
		}
	}
	for _, x := range ordenTipos {
		if !estados.IsValidTipo(x) {
			t.Errorf("tipo %s", x)
		}
	}
	for _, x := range ordenPrioridades {
		if !estados.IsValidPrioridad(x) {
			t.Errorf("prioridad %s", x)
		}
	}
	for _, x := range ordenAreas {
		if !estados.IsValidArea(x) {
			t.Errorf("área %s", x)
		}
	}
	for _, e := range repository.EstadosPendientes {
		if !estados.IsValidEstado(e) {
			t.Errorf("estado pendiente %s", e)
		}
	}
}
