package logic

import (
	"context"
	"strings"
	"testing"
	"time"

	"estudiantes/estudiantes"
	"estudiantes/internal/interceptor"
	"estudiantes/internal/repository"
	"estudiantes/internal/security"
	"estudiantes/internal/svc"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Sin base de datos ni red: el contexto se obtiene pasando un token ficticio
// por el interceptor real (claves solo de prueba).

const (
	clavePruebaUsuarios = "clave-de-usuarios-solo-para-pruebas-0001"
	clavePruebaServicio = "clave-de-servicios-solo-para-pruebas-0002"
	idSistemaPrueba     = "7b0f6d2e-1c1a-4c43-9d55-0a6a8f1b2c3d"
	idUsuarioPrueba     = "3f2b1c4d-0000-4000-8000-000000000001"
	dniPrueba           = "45127823"
)

func contextoPorInterceptor(t *testing.T, token, metodo string) context.Context {
	t.Helper()
	sv, err := security.NewServiceTokenValidator(clavePruebaServicio, clavePruebaUsuarios, "", "")
	if err != nil || sv == nil {
		t.Fatalf("validador: %v", err)
	}
	icpt := interceptor.AuthenticationInterceptor(security.NewJWTValidator(clavePruebaUsuarios), sv)
	in := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	var ctx context.Context
	if _, err := icpt(in, nil, &grpc.UnaryServerInfo{FullMethod: metodo}, func(c context.Context, _ any) (any, error) {
		ctx = c
		return nil, nil
	}); err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	return ctx
}

func contextoServicio(t *testing.T, scope, metodo string) context.Context {
	t.Helper()
	ahora := time.Now()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, security.ServiceClaims{
		TokenType: "service", ClientID: "cli_0123456789abcdef0123456789abcdef", Scope: scope,
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "tungasuca-auth", Subject: idSistemaPrueba, Audience: jwt.ClaimStrings{"estudiantes"},
			IssuedAt: jwt.NewNumericDate(ahora), ExpiresAt: jwt.NewNumericDate(ahora.Add(5 * time.Minute))},
	}).SignedString([]byte(clavePruebaServicio))
	if err != nil {
		t.Fatal(err)
	}
	return contextoPorInterceptor(t, tok, metodo)
}

func contextoUsuario(t *testing.T, metodo string) context.Context {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, security.Claims{Username: "secretaria.demo", Role: "SECRETARIA",
		RegisteredClaims: jwt.RegisteredClaims{Subject: idUsuarioPrueba, IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute))},
	}).SignedString([]byte(clavePruebaUsuarios))
	if err != nil {
		t.Fatal(err)
	}
	return contextoPorInterceptor(t, tok, metodo)
}

func estudianteCompleto() *repository.Estudiante {
	return &repository.Estudiante{ID: "11111111-1111-4111-8111-111111111111", Codigo: "EST-0001", Nombres: "Ana María", Apellidos: "Quispe Huamán",
		NumeroDocumento: dniPrueba, TipoDocumento: "DNI", CodigoSiagie: "123456789012", ApellidoPaterno: "Quispe", ApellidoMaterno: "Huamán",
		Nivel: "SECUNDARIA", Grado: "3", Seccion: "B", Turno: "MAÑANA", EstadoMatricula: "MATRICULADO", Estado: "ACTIVO", Version: 3}
}

// camposPoblados: nombres de los campos no vacíos del mensaje.
func camposPoblados(m *estudiantes.EstudianteBasico) []string {
	var out []string
	m.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		out = append(out, string(fd.Name()))
		return true
	})
	return out
}

func TestRespuestaMinimaParaSistemaSinView(t *testing.T) {
	const metodo = "/estudiantes.Estudiantes/GetEstudiantesBasic"
	for _, scope := range []string{"estudiantes.basic.view", "estudiantes.basic.view estudiantes.search.basic"} {
		got := basicoParaLlamador(contextoServicio(t, scope, metodo), estudianteCompleto())
		permitidos := map[string]bool{"id": true, "nombre_completo": true, "grado": true, "seccion": true}
		for _, c := range camposPoblados(got) {
			if !permitidos[c] {
				t.Fatalf("scope %q: campo %q no debe enviarse a un sistema consumidor", scope, c)
			}
		}
		if got.GetNombreCompleto() != "Ana María Quispe Huamán" || got.GetGrado() != "3" || got.GetSeccion() != "B" || got.GetId() == "" {
			t.Fatalf("respuesta mínima incompleta: %v", camposPoblados(got))
		}
		texto := got.String()
		for _, prohibido := range []string{dniPrueba, "123456789012", "EST-0001"} {
			if strings.Contains(texto, prohibido) {
				t.Fatal("la respuesta mínima contiene DNI, SIAGIE o código")
			}
		}
	}
}

func TestRespuestaCompletaParaUsuariosYSistemasConView(t *testing.T) {
	const metodo = "/estudiantes.Estudiantes/GetEstudiantesBasic"
	for nombre, ctx := range map[string]context.Context{
		"usuario":          contextoUsuario(t, metodo),
		"sistema con view": contextoServicio(t, "estudiantes.view", metodo),
	} {
		got := basicoParaLlamador(ctx, estudianteCompleto())
		if got.GetNumeroDocumento() != dniPrueba || got.GetCodigoSiagie() == "" || got.GetCodigo() == "" {
			t.Fatalf("%s: debe conservar la respuesta completa actual", nombre)
		}
	}
}

func TestResolverSistemaMinimoSoloPorDNI(t *testing.T) {
	const metodo = "/estudiantes.Estudiantes/ResolveEstudiante"
	ctx := contextoServicio(t, "estudiantes.basic.view", metodo)
	l := NewResolveEstudianteLogic(ctx, &svc.ServiceContext{})
	for nombre, req := range map[string]*estudiantes.ResolveEstudianteRequest{
		"codigo":     {Codigo: "EST-0001"},
		"siagie":     {CodigoSiagie: "123456789012"},
		"CE":         {TipoDocumento: "CE", NumeroDocumento: "000123456"},
		"sin tipo":   {NumeroDocumento: dniPrueba},
		"DNI+codigo": {TipoDocumento: "DNI", NumeroDocumento: dniPrueba, Codigo: "EST-0001"},
	} {
		_, err := l.ResolveEstudiante(req)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("%s: se esperaba InvalidArgument, obtuvo %v", nombre, err)
		}
		for _, valor := range []string{dniPrueba, "EST-0001", "123456789012", "000123456"} {
			if strings.Contains(err.Error(), valor) {
				t.Fatalf("%s: el error repite el valor consultado", nombre)
			}
		}
	}
	// DNI parcial: la validación de documento lo rechaza sin consultar la BD.
	if _, err := l.ResolveEstudiante(&estudiantes.ResolveEstudianteRequest{TipoDocumento: "DNI", NumeroDocumento: "451278"}); status.Code(err) != codes.InvalidArgument || strings.Contains(err.Error(), "451278") {
		t.Fatalf("DNI parcial: se esperaba InvalidArgument sin eco: %v", err)
	}
}

func TestBusquedaSeleccionRechazaUsuarios(t *testing.T) {
	const metodo = "/estudiantes.Estudiantes/BuscarEstudiantesSeleccion"
	l := NewBuscarEstudiantesSeleccionLogic(contextoUsuario(t, metodo), &svc.ServiceContext{})
	if _, err := l.BuscarEstudiantesSeleccion(&estudiantes.BuscarEstudiantesSeleccionRequest{Q: "quispe"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("un JWT humano no debe sustituir al token de servicio: %v", err)
	}
}
