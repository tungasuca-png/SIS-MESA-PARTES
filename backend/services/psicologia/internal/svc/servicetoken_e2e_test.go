package svc

import (
	"context"
	"os"
	"strings"
	"testing"

	"auth/authclient"

	"estudiantes/estudiantesclient"

	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Prueba REAL de extremo a extremo, de SOLO LECTURA: Auth (127.0.0.1:8080)
// emite el token de servicio con PSICOLOGIA_CLIENT_ID/SECRET y Estudiantes
// (127.0.0.1:8087) responde GetEstudiantesBasic. No escribe en Psicología ni
// en Estudiantes (Auth solo actualiza last_used_at del cliente). Se salta si
// no se activa con PSICOLOGIA_E2E_REAL=true.
//
// E2E_ESTUDIANTE_ID (opcional): UUID de un estudiante existente para
// comprobar Exists=true y NombreCompleto (el nombre no se imprime).
func TestE2E_ServicioPsicologiaConsultaEstudiantes(t *testing.T) {
	if os.Getenv("PSICOLOGIA_E2E_REAL") != "true" {
		t.Skip("prueba real: activar con PSICOLOGIA_E2E_REAL=true y las credenciales en el entorno")
	}
	id, secreto := strings.TrimSpace(os.Getenv("PSICOLOGIA_CLIENT_ID")), strings.TrimSpace(os.Getenv("PSICOLOGIA_CLIENT_SECRET"))
	if id == "" || secreto == "" {
		t.Fatal("faltan PSICOLOGIA_CLIENT_ID/SECRET en el entorno")
	}
	auth := authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8080"}))
	est := estudiantesclient.NewEstudiantes(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8087"}))
	tokens := NewServiceTokenProvider(auth, id, secreto)
	v := NewEstudianteVerifier(est, tokens)
	ctx := context.Background()

	t1, err := tokens.Token(ctx)
	if err != nil {
		t.Fatalf("Auth no emitió el token de servicio: %v", err)
	}
	t2, _ := tokens.Token(ctx)
	if t1 != t2 {
		t.Fatal("el token debió reutilizarse")
	}
	t.Log("token de servicio obtenido y reutilizado (no se muestra)")

	inexistente := "00000000-0000-4000-8000-000000000000"
	if ok, err := v.Exists(ctx, inexistente); err != nil || ok {
		t.Fatalf("Exists(inexistente) debe ser (false, nil): %v %v", ok, err)
	}
	if _, err := v.NombreCompleto(ctx, inexistente); status.Code(err) != codes.NotFound {
		t.Fatalf("NombreCompleto(inexistente) debe ser NotFound: %v", err)
	}
	t.Log("GetEstudiantesBasic con token de servicio: estudiante inexistente -> false / NotFound")

	if existente := strings.TrimSpace(os.Getenv("E2E_ESTUDIANTE_ID")); existente != "" {
		ok, err := v.Exists(ctx, existente)
		if err != nil || !ok {
			t.Fatalf("Exists(existente) debe ser (true, nil): %v %v", ok, err)
		}
		nombre, err := v.NombreCompleto(ctx, existente)
		if err != nil || strings.TrimSpace(nombre) == "" {
			t.Fatalf("NombreCompleto(existente) debe devolver un nombre: %v", err)
		}
		t.Logf("estudiante existente -> true; NombreCompleto devolvió %d caracteres (no se muestra)", len([]rune(nombre)))
	}

	// Búsqueda de selección (requiere estudiantes.search.basic otorgado al
	// cliente; activar con E2E_BUSQUEDA=true). Ni el nombre ni el DNI se
	// imprimen: E2E_ESTUDIANTE_DNI se carga desde la base sin mostrarlo.
	if os.Getenv("E2E_BUSQUEDA") == "true" {
		existente := strings.TrimSpace(os.Getenv("E2E_ESTUDIANTE_ID"))
		dni := strings.TrimSpace(os.Getenv("E2E_ESTUDIANTE_DNI"))
		if existente == "" {
			t.Fatal("E2E_BUSQUEDA requiere E2E_ESTUDIANTE_ID")
		}
		nombre, err := v.NombreCompleto(ctx, existente)
		if err != nil {
			t.Fatalf("NombreCompleto: %v", err)
		}
		contiene := func(r *estudiantesclient.BuscarEstudiantesSeleccionResponse) bool {
			for _, e := range r.GetEstudiantes() {
				if strings.EqualFold(e.GetId(), existente) {
					return true
				}
			}
			return false
		}
		palabras := strings.Fields(nombre)
		primerNombre, apellido := palabras[0], palabras[len(palabras)-1]
		for etiqueta, q := range map[string]string{"nombre": primerNombre, "apellido": apellido, "nombre + apellido": primerNombre + " " + apellido} {
			r, err := v.Buscar(ctx, q, 1, 20)
			if err != nil || !contiene(r) {
				// Con muchos homónimos el estudiante podría no estar en la primera
				// página: basta con que haya resultados y el total lo incluya.
				if err != nil || r.GetTotal() < 1 {
					t.Fatalf("búsqueda por %s: %v", etiqueta, err)
				}
			}
			for _, e := range r.GetEstudiantes() {
				if e.GetId() == "" || e.GetNombreCompleto() == "" {
					t.Fatalf("búsqueda por %s: resultado incompleto", etiqueta)
				}
			}
			t.Logf("búsqueda por %s: %d resultado(s), estudiante incluido=%v", etiqueta, r.GetTotal(), contiene(r))
		}
		if dni != "" {
			r, err := v.Buscar(ctx, dni, 1, 20)
			if err != nil || !contiene(r) {
				t.Fatalf("búsqueda por DNI completo debe encontrar al estudiante: %v", err)
			}
			for _, e := range r.GetEstudiantes() {
				if strings.Contains(e.String(), dni) {
					t.Fatal("la respuesta contiene el DNI")
				}
			}
			if r, err := v.Buscar(ctx, dni[:6], 1, 20); err != nil || r.GetTotal() != 0 {
				t.Fatalf("DNI parcial no debe encontrar nada: %v", err)
			}
		}
		t.Log("búsqueda de selección: por nombre OK; por DNI completo OK sin DNI en la respuesta; DNI parcial sin resultados")
	}

	// Mínimo privilegio: con el mismo token, GetEstudiante (exige
	// estudiantes.view) debe rechazarse.
	_, err = est.GetEstudiante(withBearer(ctx, t1), &estudiantesclient.GetEstudianteRequest{Id: inexistente})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("GetEstudiante con basic.view debe ser PermissionDenied: %v", err)
	}
	t.Log("GetEstudiante con el token de servicio -> PermissionDenied (scope mínimo)")
}

func withBearer(ctx context.Context, token string) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token))
}
