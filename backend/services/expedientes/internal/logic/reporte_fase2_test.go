package logic

import (
	"context"
	"strings"
	"testing"
	"time"

	"expedientes/expedientes"
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

func filasFase2() []repository.ReporteFila {
	d := func(n int) *int { return &n }
	return []repository.ReporteFila{
		{Estado: "PENDIENTE", Tipo: "CONSTANCIA", Prioridad: "NORMAL", Area: "SECRETARIA", Mes: "2026-10", DiasPendiente: d(1), Cantidad: 2, Dia: "2026-10-28", Registrador: "u-1"},
		{Estado: "ATENDIDO", Tipo: "CONSTANCIA", Prioridad: "NORMAL", Area: "DIRECTOR", Mes: "2026-10", Cantidad: 1, Dia: "2026-10-26", Registrador: "u-2"},
		{Estado: "EN_PROCESO", Tipo: "PERMISO", Prioridad: "URGENTE", Area: "DIRECTOR", Mes: "2026-10", DiasPendiente: d(4), Cantidad: 3, Dia: "2026-10-26", Registrador: "u-1"},
	}
}

func celda(cs []*expedientes.ReporteCruce, fila, col string) int32 {
	for _, c := range cs {
		if c.Fila == fila && c.Columna == col {
			return c.Cantidad
		}
	}
	return 0
}

func TestCompletarFase2_CrucesSerieYRegistros(t *testing.T) {
	filas := filasFase2()
	f := repository.ReporteFilter{Desde: "2026-10-26", Hasta: "2026-10-30", ConDia: true, ConRegistrador: true}
	r := armarReporte(filas, f, hoyPrueba)
	if err := completarFase2(r, filas, f, "DIA", hoyPrueba); err != nil {
		t.Fatal(err)
	}
	if celda(r.AreaEstado, "DIRECTOR", "EN_PROCESO") != 3 || celda(r.AreaEstado, "DIRECTOR", "ATENDIDO") != 1 || celda(r.AreaEstado, "SECRETARIA", "PENDIENTE") != 2 || len(r.AreaEstado) != 3 {
		t.Fatalf("área × estado: %v", r.AreaEstado)
	}
	if celda(r.TipoEstado, "CONSTANCIA", "PENDIENTE") != 2 || celda(r.TipoEstado, "PERMISO", "EN_PROCESO") != 3 {
		t.Fatalf("tipo × estado: %v", r.TipoEstado)
	}
	suma := int32(0)
	for _, c := range r.AreaEstado {
		suma += c.Cantidad
	}
	if suma != r.Total {
		t.Fatalf("suma de cruces %d != total %d", suma, r.Total)
	}
	if len(r.Serie) != 5 || r.Serie[0].Clave != "2026-10-26" || r.Serie[0].Cantidad != 4 || r.Serie[2].Cantidad != 2 || r.Granularidad != "DIA" {
		t.Fatalf("serie diaria: %v", r.Serie)
	}
	// Evolución por estado: cada periodo suma lo mismo que la serie y sus
	// claves son las de la serie (en el mismo orden).
	porPeriodo := map[string]int32{}
	orden := []string{}
	for _, c := range r.SerieEstado {
		if c.Cantidad <= 0 {
			t.Fatalf("celda sin valor: %v", c)
		}
		if len(orden) == 0 || orden[len(orden)-1] != c.Fila {
			orden = append(orden, c.Fila)
		}
		porPeriodo[c.Fila] += c.Cantidad
	}
	for _, c := range r.Serie {
		if porPeriodo[c.Clave] != c.Cantidad {
			t.Fatalf("periodo %s: estados %d != serie %d", c.Clave, porPeriodo[c.Clave], c.Cantidad)
		}
	}
	if strings.Join(orden, ",") != "2026-10-26,2026-10-28" || celda(r.SerieEstado, "2026-10-26", "EN_PROCESO") != 3 {
		t.Fatalf("serie por estado: %v", r.SerieEstado)
	}
	if len(r.RegistrosPorUsuario) != 2 || r.RegistrosPorUsuario[0].Clave != "u-1" || r.RegistrosPorUsuario[0].Cantidad != 5 {
		t.Fatalf("registros: %v", r.RegistrosPorUsuario)
	}

	rs := armarReporte(filas, f, hoyPrueba)
	_ = completarFase2(rs, filas, f, "SEMANA", hoyPrueba)
	if len(rs.Serie) != 1 || rs.Serie[0].Clave != "2026-10-26" || rs.Serie[0].Cantidad != 6 {
		t.Fatalf("semana: %v", rs.Serie)
	}
	if celda(rs.SerieEstado, "2026-10-26", "PENDIENTE") != 2 || celda(rs.SerieEstado, "2026-10-26", "EN_PROCESO") != 3 ||
		celda(rs.SerieEstado, "2026-10-26", "ATENDIDO") != 1 || len(rs.SerieEstado) != 3 {
		t.Fatalf("evolución por estado (semana): %v", rs.SerieEstado)
	}
	rm := armarReporte(filas, f, hoyPrueba)
	_ = completarFase2(rm, filas, f, "MES", hoyPrueba)
	if len(rm.Serie) != 1 || rm.Serie[0].Clave != "2026-10" || rm.Serie[0].Cantidad != 6 {
		t.Fatalf("mes: %v", rm.Serie)
	}
}

func TestCompletarFase2_SinOpcionalesNoCambiaFase1(t *testing.T) {
	filas := filasFase2()
	for i := range filas {
		filas[i].Dia, filas[i].Registrador = "", ""
	}
	f := repository.ReporteFilter{}
	base := armarReporte(filas, f, hoyPrueba)
	r := armarReporte(filas, f, hoyPrueba)
	if err := completarFase2(r, filas, f, "", hoyPrueba); err != nil {
		t.Fatal(err)
	}
	if len(r.Serie) != 0 || len(r.SerieEstado) != 0 || r.Granularidad != "" || len(r.RegistrosPorUsuario) != 0 {
		t.Fatal("sin opcionales no hay serie ni registros")
	}
	if r.Total != base.Total || len(r.PorMes) != len(base.PorMes) || r.Pendientes != base.Pendientes {
		t.Fatal("los campos de la Fase 1 no deben cambiar")
	}
}

func TestCompletarFase2_SerieDemasiadoLarga(t *testing.T) {
	f := repository.ReporteFilter{Desde: "2024-01-01", Hasta: "2026-10-30"}
	r := armarReporte(nil, f, hoyPrueba)
	if err := completarFase2(r, nil, f, "DIA", hoyPrueba); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("serie diaria de casi 3 años: %v", err)
	}
}

func alcanceComo(t *testing.T, auth *authFalso, role string, in *expedientes.ReporteAlcanceRequest) error {
	t.Helper()
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, security.Claims{Role: role, RegisteredClaims: jwt.RegisteredClaims{
		Subject: "00000000-0000-4000-8000-000000000001", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}).SignedString([]byte(secretoListado))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tok))
	_, err := interceptor.AuthenticationInterceptor(security.NewJWTValidator(secretoListado))(ctx, in, &grpc.UnaryServerInfo{},
		func(ctx context.Context, req any) (any, error) {
			return NewReporteAlcanceExpedientesLogic(ctx, &svc.ServiceContext{AuthClient: auth}).ReporteAlcanceExpedientes(req.(*expedientes.ReporteAlcanceRequest))
		})
	return err
}

func TestReporteAlcance_PermisosYValidacion(t *testing.T) {
	if err := alcanceComo(t, &authFalso{permitido: map[string]bool{"expedientes.view": true}}, "SECRETARIA", &expedientes.ReporteAlcanceRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("sin reportes.view: %v", err)
	}
	con := &authFalso{permitido: map[string]bool{"reportes.view": true}}
	if err := alcanceComo(t, con, "SOLICITANTE", &expedientes.ReporteAlcanceRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("rol externo: %v", err)
	}
	err := alcanceComo(t, con, "ADMIN", &expedientes.ReporteAlcanceRequest{Tipo: "X' OR 1=1"})
	if status.Code(err) != codes.InvalidArgument || strings.Contains(err.Error(), "OR 1=1") {
		t.Fatalf("tipo inválido sin eco: %v", err)
	}
}

func TestReporte_GranularidadInvalida(t *testing.T) {
	auth := &authFalso{permitido: map[string]bool{"reportes.view": true}}
	if err := reporteComo(t, auth, "SECRETARIA", &expedientes.ReporteExpedientesRequest{Granularidad: "ANIO"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("granularidad inválida: %v", err)
	}
}
