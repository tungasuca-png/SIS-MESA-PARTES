package logic

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"estudiantes/estudiantesclient"
	"psicologia/internal/svc"
	"psicologia/psicologia"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type resumidorFalso struct {
	llamadas int
	ids      []string
	nombres  map[string]string
	err      error
}

func (r *resumidorFalso) Resumenes(_ context.Context, ids []string) (map[string]string, error) {
	r.llamadas++
	r.ids = ids
	return r.nombres, r.err
}

const (
	estA = "11111111-1111-4111-8111-111111111111"
	estB = "22222222-2222-4222-8222-222222222222"
)

func TestCompletarEstudiantes(t *testing.T) {
	r := &resumidorFalso{nombres: map[string]string{estA: "Ana María Quispe"}}
	sc := &svc.ServiceContext{EstudiantesResumen: r}
	exps := []*psicologia.ExpedientePsicologico{{EstudianteId: estA}, {EstudianteId: strings.ToUpper(estA)}, {EstudianteId: estB}}
	completarEstudiantes(context.Background(), sc, exps...)
	if r.llamadas != 1 {
		t.Fatalf("debe haber UNA consulta por respuesta, hubo %d", r.llamadas)
	}
	for i, esperado := range []string{resolucionResuelto, resolucionResuelto, resolucionNoEncontrado} {
		if exps[i].Estudiante.GetResolucion() != esperado {
			t.Fatalf("expediente %d: %v", i, exps[i].Estudiante)
		}
	}
	if exps[0].Estudiante.GetNombreCompleto() != "Ana María Quispe" || exps[2].Estudiante.GetNombreCompleto() != "" {
		t.Fatal("nombre inesperado")
	}

	// Estudiantes o Auth no disponibles: el expediente se entrega igual.
	caido := []*psicologia.ExpedientePsicologico{{EstudianteId: estA}}
	completarEstudiantes(context.Background(), &svc.ServiceContext{EstudiantesResumen: &resumidorFalso{err: status.Error(codes.Unavailable, "x")}}, caido...)
	if caido[0].Estudiante.GetResolucion() != resolucionNoDisponible || caido[0].Estudiante.GetNombreCompleto() != "" {
		t.Fatalf("se esperaba NO_DISPONIBLE: %v", caido[0].Estudiante)
	}
	sinConfig := []*psicologia.ExpedientePsicologico{{EstudianteId: estA}}
	completarEstudiantes(context.Background(), &svc.ServiceContext{}, sinConfig...)
	if sinConfig[0].Estudiante.GetResolucion() != resolucionNoDisponible {
		t.Fatal("sin resumidor configurado debe ser NO_DISPONIBLE")
	}
}

type buscadorFalso struct {
	llamadas int
	q        string
	err      error
}

func (b *buscadorFalso) Buscar(_ context.Context, q string, _, _ int32) (*estudiantesclient.BuscarEstudiantesSeleccionResponse, error) {
	b.llamadas++
	b.q = q
	if b.err != nil {
		return nil, b.err
	}
	return &estudiantesclient.BuscarEstudiantesSeleccionResponse{Total: 1, Estudiantes: []*estudiantesclient.EstudianteSeleccion{
		{Id: estA, NombreCompleto: "Ana María Quispe", Grado: "3", Seccion: "B"},
	}}, nil
}

func TestBuscarEstudiantes_PermisosHumanosAntesDeConsultar(t *testing.T) {
	casos := []struct {
		nombre    string
		permisos  map[string]bool
		authErr   error
		esperado  codes.Code
		consultar bool
	}{
		{"sin permisos de Psicología", map[string]bool{"estudiantes.view": true}, nil, codes.PermissionDenied, false},
		{"Auth no disponible", nil, status.Error(codes.Unavailable, "auth"), codes.Unavailable, false},
		{"con create", map[string]bool{permExpedientesCreate: true}, nil, codes.OK, true},
		{"con view", map[string]bool{permExpedientesView: true}, nil, codes.OK, true},
		{"con view_own", map[string]bool{permExpedientesViewOwn: true}, nil, codes.OK, true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			b := &buscadorFalso{}
			sc := &svc.ServiceContext{AuthClient: &authPermisos{permitidos: c.permisos, err: c.authErr}, EstudiantesBuscador: b, AuditoriaRepo: bitacoraSinBase(t)}
			resp, err := NewBuscarEstudiantesLogic(ctxPsicologo(t), sc).BuscarEstudiantes(&psicologia.BuscarEstudiantesRequest{Q: "quispe"})
			if status.Code(err) != c.esperado || (b.llamadas > 0) != c.consultar {
				t.Fatalf("err=%v consultas=%d", err, b.llamadas)
			}
			if c.consultar {
				e := resp.GetEstudiantes()[0]
				if resp.GetTotal() != 1 || e.GetId() != estA || e.GetNombreCompleto() != "Ana María Quispe" || e.GetGrado() != "3" || e.GetSeccion() != "B" {
					t.Fatalf("respuesta inesperada: %v", resp)
				}
			}
		})
	}
}

func TestBuscarEstudiantes_ValidacionYErrores(t *testing.T) {
	permitido := map[string]bool{permExpedientesView: true}
	b := &buscadorFalso{}
	sc := &svc.ServiceContext{AuthClient: &authPermisos{permitidos: permitido}, EstudiantesBuscador: b, AuditoriaRepo: bitacoraSinBase(t)}
	for _, q := range []string{"", " a ", strings.Repeat("x", 101)} {
		if _, err := NewBuscarEstudiantesLogic(ctxPsicologo(t), sc).BuscarEstudiantes(&psicologia.BuscarEstudiantesRequest{Q: q}); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("%q: se esperaba InvalidArgument: %v", q, err)
		}
	}
	if b.llamadas != 0 {
		t.Fatal("una consulta inválida no debe llegar a Estudiantes")
	}
	caido := &svc.ServiceContext{AuthClient: &authPermisos{permitidos: permitido}, EstudiantesBuscador: &buscadorFalso{err: status.Error(codes.Unavailable, "x")}, AuditoriaRepo: bitacoraSinBase(t)}
	if _, err := NewBuscarEstudiantesLogic(ctxPsicologo(t), caido).BuscarEstudiantes(&psicologia.BuscarEstudiantesRequest{Q: "quispe"}); status.Code(err) != codes.Unavailable {
		t.Fatalf("Estudiantes caído debe ser Unavailable: %v", err)
	}
	sinBuscador := &svc.ServiceContext{AuthClient: &authPermisos{permitidos: permitido}, AuditoriaRepo: bitacoraSinBase(t)}
	if _, err := NewBuscarEstudiantesLogic(ctxPsicologo(t), sinBuscador).BuscarEstudiantes(&psicologia.BuscarEstudiantesRequest{Q: "quispe"}); status.Code(err) != codes.Unavailable {
		t.Fatalf("sin buscador configurado debe ser Unavailable: %v", err)
	}
}

func TestBuscarEstudiantes_ConsultaNuncaEnLogs(t *testing.T) {
	var buf bytes.Buffer
	logx.SetWriter(logx.NewWriter(&buf))
	defer logx.Reset()
	const dni = "45127823"
	permitido := map[string]bool{permExpedientesView: true}
	for _, sc := range []*svc.ServiceContext{
		{AuthClient: &authPermisos{permitidos: permitido}, EstudiantesBuscador: &buscadorFalso{}, AuditoriaRepo: bitacoraSinBase(t)},
		{AuthClient: &authPermisos{permitidos: permitido}, EstudiantesBuscador: &buscadorFalso{err: status.Error(codes.Unavailable, "x")}, AuditoriaRepo: bitacoraSinBase(t)},
		{AuthClient: &authPermisos{permitidos: map[string]bool{}}, EstudiantesBuscador: &buscadorFalso{}, AuditoriaRepo: bitacoraSinBase(t)},
	} {
		_, _ = NewBuscarEstudiantesLogic(ctxPsicologo(t), sc).BuscarEstudiantes(&psicologia.BuscarEstudiantesRequest{Q: "quispe " + dni})
	}
	if strings.Contains(buf.String(), dni) || strings.Contains(buf.String(), "quispe") {
		t.Fatalf("la consulta quedó en el log: %s", buf.String())
	}
}
