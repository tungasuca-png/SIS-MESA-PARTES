package logic

import (
	"bytes"
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"auth/internal/interceptor"
	"auth/internal/repository"
	"auth/internal/security"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Otorgar / retirar permisos de sistemas consumidores, sin base de datos: el
// repositorio es un doble en memoria con su propio "catálogo". Identificadores
// y credenciales ficticios.

const adminPrueba = "3f2b1c4d-0000-4000-8000-0000000000aa"

type permisosFalso struct {
	clientes  map[string]*repository.ServiceClient // por client_id
	catalogo  map[string]bool
	asignados map[string]map[string]bool // id del cliente -> permisos
	// rolePermisos simula role_permissions: nunca debe cambiar.
	rolePermisos map[string][]string
}

func nuevoPermisosFalso(t *testing.T, estado string, permisos ...string) (*permisosFalso, string) {
	t.Helper()
	clientID, _, hash, err := security.GenerateClientCredentials()
	if err != nil {
		t.Fatal(err)
	}
	f := &permisosFalso{
		clientes:     map[string]*repository.ServiceClient{},
		catalogo:     map[string]bool{"estudiantes.basic.view": true, "estudiantes.search.basic": true, "estudiantes.view": true, "roles.view": true},
		asignados:    map[string]map[string]bool{},
		rolePermisos: map[string][]string{"PSICOLOGO": {"psicologia.expedientes.view"}},
	}
	id := "7b0f6d2e-1c1a-4c43-9d55-0a6a8f1b2c3d"
	f.clientes[clientID] = &repository.ServiceClient{ID: id, Nombre: "Servicio de prueba", ClientID: clientID, SecretHash: hash, Estado: estado}
	f.asignados[id] = map[string]bool{}
	for _, p := range permisos {
		f.asignados[id][p] = true
	}
	return f, clientID
}

func (f *permisosFalso) FindByClientID(_ context.Context, clientID string) (*repository.ServiceClient, error) {
	c, ok := f.clientes[clientID]
	if !ok {
		return nil, repository.ErrServiceClientNotFound
	}
	copia := *c
	copia.Permisos = nil
	for p := range f.asignados[c.ID] {
		copia.Permisos = append(copia.Permisos, p)
	}
	sort.Strings(copia.Permisos)
	return &copia, nil
}

func (f *permisosFalso) AddPermission(_ context.Context, id, codigo string) (bool, error) {
	if !f.catalogo[codigo] {
		return false, repository.ErrPermisoNoCatalogado
	}
	if f.asignados[id][codigo] {
		return false, nil
	}
	f.asignados[id][codigo] = true
	return true, nil
}

func (f *permisosFalso) RemovePermission(_ context.Context, id, codigo string) (bool, error) {
	if !f.asignados[id][codigo] {
		return false, nil
	}
	delete(f.asignados[id], codigo)
	return true, nil
}

func otorgar(f *permisosFalso, clientID, permiso string) (bool, []string, error) {
	r, err := cambiarPermisoSistema(context.Background(), f, adminPrueba, clientID, permiso, true)
	if err != nil {
		return false, nil, err
	}
	return r.Changed, r.Client.Permisos, nil
}

func retirar(f *permisosFalso, clientID, permiso string) (bool, []string, error) {
	r, err := cambiarPermisoSistema(context.Background(), f, adminPrueba, clientID, permiso, false)
	if err != nil {
		return false, nil, err
	}
	return r.Changed, r.Client.Permisos, nil
}

func TestPermisoSistema_OtorgarYRetirarCorrectos(t *testing.T) {
	f, cid := nuevoPermisosFalso(t, repository.ServiceClientActivo, "estudiantes.basic.view")
	cambio, permisos, err := otorgar(f, cid, "estudiantes.search.basic")
	if err != nil || !cambio || strings.Join(permisos, " ") != "estudiantes.basic.view estudiantes.search.basic" {
		t.Fatalf("otorgar: cambio=%v permisos=%v err=%v", cambio, permisos, err)
	}
	cambio, permisos, err = retirar(f, cid, "estudiantes.search.basic")
	if err != nil || !cambio || strings.Join(permisos, " ") != "estudiantes.basic.view" {
		t.Fatalf("retirar: cambio=%v permisos=%v err=%v", cambio, permisos, err)
	}
}

func TestPermisoSistema_Idempotencia(t *testing.T) {
	f, cid := nuevoPermisosFalso(t, repository.ServiceClientActivo, "estudiantes.basic.view")
	if c, _, _ := otorgar(f, cid, "estudiantes.search.basic"); !c {
		t.Fatal("el primer otorgamiento debe cambiar")
	}
	cambio, permisos, err := otorgar(f, cid, "estudiantes.search.basic")
	if err != nil || cambio || len(permisos) != 2 {
		t.Fatalf("otorgamiento duplicado: debe responder bien sin duplicar: cambio=%v permisos=%v err=%v", cambio, permisos, err)
	}
	if c, _, _ := retirar(f, cid, "estudiantes.search.basic"); !c {
		t.Fatal("el primer retiro debe cambiar")
	}
	cambio, permisos, err = retirar(f, cid, "estudiantes.search.basic")
	if err != nil || cambio || strings.Join(permisos, " ") != "estudiantes.basic.view" {
		t.Fatalf("retiro de un permiso que ya no tiene: debe responder bien: cambio=%v permisos=%v err=%v", cambio, permisos, err)
	}
}

func TestPermisoSistema_Rechazos(t *testing.T) {
	activo, cidActivo := nuevoPermisosFalso(t, repository.ServiceClientActivo)
	revocado, cidRevocado := nuevoPermisosFalso(t, repository.ServiceClientRevocado, "estudiantes.search.basic")
	inexistente, _, _, _ := security.GenerateClientCredentials()
	sinCatalogo, cidSinCatalogo := nuevoPermisosFalso(t, repository.ServiceClientActivo)
	delete(sinCatalogo.catalogo, "estudiantes.search.basic") // migración 020 aún no aplicada

	casos := []struct {
		nombre   string
		f        *permisosFalso
		cid      string
		permiso  string
		otorga   bool
		esperado codes.Code
	}{
		{"cliente inexistente (otorgar)", activo, inexistente, "estudiantes.search.basic", true, codes.NotFound},
		{"cliente inexistente (retirar)", activo, inexistente, "estudiantes.search.basic", false, codes.NotFound},
		{"cliente revocado (otorgar)", revocado, cidRevocado, "estudiantes.basic.view", true, codes.FailedPrecondition},
		{"permiso no permitido", activo, cidActivo, "expedientes.view", true, codes.InvalidArgument},
		{"estudiantes.view (otorgar)", activo, cidActivo, "estudiantes.view", true, codes.InvalidArgument},
		{"estudiantes.view (retirar)", activo, cidActivo, "estudiantes.view", false, codes.InvalidArgument},
		{"roles.view", activo, cidActivo, "roles.view", true, codes.InvalidArgument},
		{"integraciones.manage", activo, cidActivo, "integraciones.manage", true, codes.InvalidArgument},
		{"permiso vacío", activo, cidActivo, "", true, codes.InvalidArgument},
		{"client_id inválido", activo, "no-es-un-client-id", "estudiantes.search.basic", true, codes.InvalidArgument},
		{"permiso fuera del catálogo", sinCatalogo, cidSinCatalogo, "estudiantes.search.basic", true, codes.FailedPrecondition},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			antes := len(c.f.asignados["7b0f6d2e-1c1a-4c43-9d55-0a6a8f1b2c3d"])
			_, err := cambiarPermisoSistema(context.Background(), c.f, adminPrueba, c.cid, c.permiso, c.otorga)
			if status.Code(err) != c.esperado {
				t.Fatalf("se esperaba %v: %v", c.esperado, err)
			}
			if len(c.f.asignados["7b0f6d2e-1c1a-4c43-9d55-0a6a8f1b2c3d"]) != antes {
				t.Fatal("un rechazo no debe cambiar los permisos")
			}
		})
	}
	// A un cliente revocado sí se le puede retirar un permiso (reduce acceso).
	if cambio, _, err := retirar(revocado, cidRevocado, "estudiantes.search.basic"); err != nil || !cambio {
		t.Fatalf("retirar de un cliente revocado debe permitirse: %v %v", cambio, err)
	}
}

func TestPermisoSistema_AislamientoDeRolesHumanos(t *testing.T) {
	f, cid := nuevoPermisosFalso(t, repository.ServiceClientActivo)
	antes := strings.Join(f.rolePermisos["PSICOLOGO"], ",")
	_, _, _ = otorgar(f, cid, "estudiantes.search.basic")
	_, _, _ = retirar(f, cid, "estudiantes.search.basic")
	if strings.Join(f.rolePermisos["PSICOLOGO"], ",") != antes {
		t.Fatal("otorgar/retirar a un sistema no debe tocar permisos de roles")
	}
}

func TestPermisoSistema_Auditoria(t *testing.T) {
	var buf bytes.Buffer
	logx.SetWriter(logx.NewWriter(&buf))
	defer logx.Reset()

	f, cid := nuevoPermisosFalso(t, repository.ServiceClientActivo)
	secreto := "cs_secreto-que-nunca-debe-aparecer-en-logs-0123456789"
	_, _, _ = otorgar(f, cid, "estudiantes.search.basic")
	_, _, _ = otorgar(f, cid, "estudiantes.search.basic")
	_, _, _ = retirar(f, cid, "estudiantes.search.basic")
	_, _, _ = otorgar(f, cid, "estudiantes.view")
	_, _ = cambiarPermisoSistema(context.Background(), f, adminPrueba, secreto, "estudiantes.search.basic", true)

	salida := buf.String()
	lineas := strings.Count(salida, "auditoría: permiso de sistema consumidor")
	if lineas != 5 {
		t.Fatalf("se esperaban 5 registros de auditoría, hubo %d: %s", lineas, salida)
	}
	for _, esperado := range []string{`"operacion":"otorgar"`, `"operacion":"retirar"`, `"resultado":"aplicado"`,
		`"resultado":"sin cambios (idempotente)"`, `"resultado":"rechazado: permiso no asignable"`,
		`"admin_id":"` + adminPrueba + `"`, `"client_id":"` + cid + `"`, `"permiso":"estudiantes.search.basic"`} {
		if !strings.Contains(salida, esperado) {
			t.Fatalf("falta %s en la auditoría: %s", esperado, salida)
		}
	}
	if strings.Contains(salida, secreto) {
		t.Fatal("un secreto pegado como client_id quedó en el log")
	}
}

// Autorización real de Auth (interceptores): sin token válido, 401; sin
// integraciones.manage, 403; con el permiso, la operación se ejecuta.
type checkerFalso struct{ permitido bool }

func (c checkerFalso) HasPermission(context.Context, string, string) (bool, error) {
	return c.permitido, nil
}

func TestPermisoSistema_AutorizacionDeLosRPC(t *testing.T) {
	const clave = "clave-de-usuarios-solo-para-pruebas-auth-0001"
	jwtm := security.NewJWTManager(clave, 15*time.Minute)
	tok, _, err := jwtm.GenerateAccessToken(adminPrueba, "admin.demo", "ADMIN")
	if err != nil {
		t.Fatal(err)
	}
	politicas := interceptor.DefaultMethodPolicies()
	for _, metodo := range []string{"/auth.Auth/GrantServiceClientPermission", "/auth.Auth/WithdrawServiceClientPermission"} {
		if p := politicas[metodo]; p.Public || p.Permission != "integraciones.manage" {
			t.Fatalf("%s debe exigir integraciones.manage: %+v", metodo, p)
		}
		llamar := func(token string, permitido bool) (bool, error) {
			ctx := context.Background()
			if token != "" {
				ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token))
			}
			ejecutado := false
			info := &grpc.UnaryServerInfo{FullMethod: metodo}
			authz := interceptor.AuthorizationInterceptor(checkerFalso{permitido}, politicas)
			_, err := interceptor.AuthenticationInterceptor(jwtm, politicas)(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
				return authz(ctx, req, info, func(context.Context, any) (any, error) { ejecutado = true; return nil, nil })
			})
			return ejecutado, err
		}
		if ej, err := llamar("token.invalido.x", true); status.Code(err) != codes.Unauthenticated || ej {
			t.Fatalf("%s con token inválido: %v", metodo, err)
		}
		if ej, err := llamar("", true); status.Code(err) != codes.Unauthenticated || ej {
			t.Fatalf("%s sin token: %v", metodo, err)
		}
		if ej, err := llamar(tok, false); status.Code(err) != codes.PermissionDenied || ej {
			t.Fatalf("%s sin integraciones.manage: %v", metodo, err)
		}
		if ej, err := llamar(tok, true); err != nil || !ej {
			t.Fatalf("%s con integraciones.manage: %v", metodo, err)
		}
	}
}

func TestPermisoSistema_ListaOtorgable(t *testing.T) {
	if !security.AssignableServicePermissions["estudiantes.search.basic"] || !security.GrantableServicePermissions["estudiantes.search.basic"] {
		t.Fatal("estudiantes.search.basic debe poder asignarse y otorgarse a sistemas")
	}
	for p := range security.GrantableServicePermissions {
		if !security.AssignableServicePermissions[p] {
			t.Fatalf("%s es otorgable pero no asignable: la lista otorgable debe ser un subconjunto", p)
		}
	}
	for _, p := range []string{"estudiantes.view", "estudiantes.personal.view", "estudiantes.create", "estudiantes.update",
		"estudiantes.matricula.update", "roles.view", "integraciones.manage", "usuarios.change_role"} {
		if security.GrantableServicePermissions[p] {
			t.Fatalf("%s no debe poder otorgarse a un sistema existente", p)
		}
	}
}
