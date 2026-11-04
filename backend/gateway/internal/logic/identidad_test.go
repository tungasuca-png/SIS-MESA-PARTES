package logic

import (
	"context"
	"testing"

	"gateway/internal/svc"
	"gateway/internal/types"

	"auth/authclient"
	"usuarios/usuariosclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Orquestación de la verificación de identidad entre Auth y Usuarios (bases
// separadas): Auth verifica y guarda el estado; Usuarios actualiza el perfil;
// Auth confirma, registra el fallo o revierte (compensación).

type authIdentidadFalso struct {
	authclient.Auth
	identidad   *authclient.SolicitanteIdentity
	pendientes  []*authclient.SolicitanteIdentity
	totalPend   int32
	verifyErr   error
	confirmErr  error
	revertErr   error
	verificados int
	confirmados []string
	revertidos  []string
	fallos      []string
}

func (a *authIdentidadFalso) GetSolicitanteIdentity(_ context.Context, _ *authclient.GetSolicitanteIdentityRequest, _ ...grpc.CallOption) (*authclient.GetSolicitanteIdentityResponse, error) {
	return &authclient.GetSolicitanteIdentityResponse{Identity: a.identidad}, nil
}

func (a *authIdentidadFalso) VerifySolicitanteIdentity(_ context.Context, in *authclient.VerifySolicitanteIdentityRequest, _ ...grpc.CallOption) (*authclient.VerifySolicitanteIdentityResponse, error) {
	if a.verifyErr != nil {
		return nil, a.verifyErr
	}
	if in.DniEsperado != a.identidad.DniDeclarado {
		return nil, status.Error(codes.FailedPrecondition, "dni distinto")
	}
	a.verificados++
	return &authclient.VerifySolicitanteIdentityResponse{Estado: "VERIFICADA", Sincronizacion: "PERFIL_PENDIENTE"}, nil
}

func (a *authIdentidadFalso) ConfirmSolicitanteIdentitySync(_ context.Context, in *authclient.IdentitySyncRequest, _ ...grpc.CallOption) (*authclient.IdentitySyncResponse, error) {
	if a.confirmErr != nil {
		return nil, a.confirmErr
	}
	a.confirmados = append(a.confirmados, in.UserId+":"+in.Dni)
	return &authclient.IdentitySyncResponse{Estado: "VERIFICADA", Sincronizacion: "COMPLETA"}, nil
}

func (a *authIdentidadFalso) RevertSolicitanteIdentity(_ context.Context, in *authclient.IdentitySyncRequest, _ ...grpc.CallOption) (*authclient.IdentitySyncResponse, error) {
	if a.revertErr != nil {
		return nil, a.revertErr
	}
	a.revertidos = append(a.revertidos, in.UserId+":"+in.Dni)
	return &authclient.IdentitySyncResponse{Estado: "PENDIENTE"}, nil
}

func (a *authIdentidadFalso) RecordSolicitanteIdentitySyncFailure(_ context.Context, in *authclient.IdentitySyncFailureRequest, _ ...grpc.CallOption) (*authclient.IdentitySyncResponse, error) {
	a.fallos = append(a.fallos, in.UserId+":"+in.Codigo)
	return &authclient.IdentitySyncResponse{Estado: "VERIFICADA", Sincronizacion: "PERFIL_PENDIENTE"}, nil
}

func (a *authIdentidadFalso) ListSolicitanteIdentities(_ context.Context, in *authclient.ListSolicitanteIdentitiesRequest, _ ...grpc.CallOption) (*authclient.ListSolicitanteIdentitiesResponse, error) {
	if !in.SincronizacionPendiente || in.Estado != "VERIFICADA" {
		return nil, status.Error(codes.InvalidArgument, "la reconciliación pide solo verificadas pendientes")
	}
	return &authclient.ListSolicitanteIdentitiesResponse{Items: a.pendientes, Total: a.totalPend}, nil
}

type usuariosIdentidadFalso struct {
	usuariosclient.Usuarios
	asignarErr map[string]error // por usuario (vacío = todos bien)
	errTodos   error
	asignados  []string
}

func (u *usuariosIdentidadFalso) AssignVerifiedDni(_ context.Context, in *usuariosclient.VerifiedDniRequest, _ ...grpc.CallOption) (*usuariosclient.VerifiedDniResponse, error) {
	if err := u.asignarErr[in.Id]; err != nil {
		return nil, err
	}
	if u.errTodos != nil {
		return nil, u.errTodos
	}
	u.asignados = append(u.asignados, in.Id+":"+in.Dni)
	return &usuariosclient.VerifiedDniResponse{}, nil
}

func pendiente() *authclient.SolicitanteIdentity {
	return &authclient.SolicitanteIdentity{UserId: "u1", DniDeclarado: "12345678", Estado: "PENDIENTE"}
}

func verificarCon(a *authIdentidadFalso, u *usuariosIdentidadFalso, metodo string) (*types.IdentidadEstadoResponse, error) {
	return NewVerificarIdentidadLogic(context.Background(), &svc.ServiceContext{AuthClient: a, UsuariosClient: u}).
		VerificarIdentidad(&types.VerificarIdentidadRequest{Id: "u1", Metodo: metodo})
}

func TestVerificarIdentidad_AmbosServiciosCompletan(t *testing.T) {
	a, u := &authIdentidadFalso{identidad: pendiente()}, &usuariosIdentidadFalso{}
	r, err := verificarCon(a, u, "presencial")
	if err != nil || r.Estado != "VERIFICADA" || r.Sincronizacion != "COMPLETA" ||
		a.verificados != 1 || len(u.asignados) != 1 || len(a.confirmados) != 1 || len(a.revertidos) != 0 || len(a.fallos) != 0 {
		t.Fatalf("éxito: %+v %v %+v %+v", r, err, a, u)
	}
}

func TestVerificarIdentidad_AuthFallaAntesDeAsignar(t *testing.T) {
	// DNI de otra cuenta en Auth: nada cambia en Usuarios.
	a := &authIdentidadFalso{identidad: pendiente(), verifyErr: status.Error(codes.AlreadyExists, "identidad_conflicto: x")}
	u := &usuariosIdentidadFalso{}
	if _, err := verificarCon(a, u, "DOCUMENTO"); status.Code(err) != codes.AlreadyExists || len(u.asignados) != 0 || len(a.fallos) != 0 {
		t.Fatalf("Auth rechaza: %v %+v", err, u)
	}
	// Conflicto ya conocido: ni siquiera se llama a Auth.Verify.
	c := pendiente()
	c.Conflicto = true
	a = &authIdentidadFalso{identidad: c}
	if _, err := verificarCon(a, u, "DOCUMENTO"); status.Code(err) != codes.AlreadyExists || a.verificados != 0 || len(u.asignados) != 0 {
		t.Fatalf("conflicto previo: %v", err)
	}
	// Método inválido: no toca ningún servicio.
	a = &authIdentidadFalso{identidad: pendiente()}
	if _, err := verificarCon(a, u, "telepatia"); status.Code(err) != codes.InvalidArgument || a.verificados != 0 {
		t.Fatalf("método inválido: %v", err)
	}
}

func TestVerificarIdentidad_UsuariosNoDisponibleQuedaPendiente(t *testing.T) {
	a := &authIdentidadFalso{identidad: pendiente()}
	u := &usuariosIdentidadFalso{errTodos: status.Error(codes.Unavailable, "caído")}
	r, err := verificarCon(a, u, "DOCUMENTO")
	if err != nil || r.Sincronizacion != "PERFIL_PENDIENTE" || len(a.confirmados) != 0 || len(a.revertidos) != 0 ||
		len(a.fallos) != 1 || a.fallos[0] != "u1:usuarios_no_disponible" {
		t.Fatalf("Usuarios caído: %+v %v %+v", r, err, a)
	}
}

func TestVerificarIdentidad_ConflictoEnPerfilCompensaAuth(t *testing.T) {
	// Auth asignó; Usuarios encuentra el DNI en otro perfil: Auth revierte.
	a := &authIdentidadFalso{identidad: pendiente()}
	u := &usuariosIdentidadFalso{errTodos: status.Error(codes.AlreadyExists, "identidad_conflicto: x")}
	if _, err := verificarCon(a, u, "DOCUMENTO"); status.Code(err) != codes.AlreadyExists ||
		len(a.revertidos) != 1 || a.revertidos[0] != "u1:12345678" || len(a.confirmados) != 0 {
		t.Fatalf("compensación: %v %+v", err, a)
	}
	// La compensación también falla: queda pendiente de reconciliación, con traza.
	a = &authIdentidadFalso{identidad: pendiente(), revertErr: status.Error(codes.Unavailable, "auth caído")}
	_, err := verificarCon(a, u, "DOCUMENTO")
	if status.Code(err) != codes.AlreadyExists || len(a.fallos) != 1 || a.fallos[0] != "u1:conflicto_perfil_reversion_pendiente" {
		t.Fatalf("compensación fallida: %v %+v", err, a)
	}
}

func TestVerificarIdentidad_ConfirmacionFallidaQuedaPendiente(t *testing.T) {
	// Usuarios ya tiene el DNI; Auth no confirma: la reconciliación lo cierra.
	a := &authIdentidadFalso{identidad: pendiente(), confirmErr: status.Error(codes.Unavailable, "auth caído")}
	u := &usuariosIdentidadFalso{}
	r, err := verificarCon(a, u, "DOCUMENTO")
	if err != nil || r.Sincronizacion != "PERFIL_PENDIENTE" || len(u.asignados) != 1 || len(a.revertidos) != 0 {
		t.Fatalf("confirmación fallida: %+v %v", r, err)
	}
}

func TestVerificarIdentidad_RepetirEsIdempotente(t *testing.T) {
	// Completa: no llama a nadie.
	c := pendiente()
	c.Estado, c.Sincronizacion = "VERIFICADA", "COMPLETA"
	a, u := &authIdentidadFalso{identidad: c}, &usuariosIdentidadFalso{}
	if r, err := verificarCon(a, u, "DOCUMENTO"); err != nil || r.Sincronizacion != "COMPLETA" || a.verificados != 0 || len(u.asignados) != 0 {
		t.Fatalf("repetir una completa: %v", err)
	}
	// Con el perfil pendiente: no vuelve a verificar en Auth, reanuda Usuarios.
	p := pendiente()
	p.Estado, p.Sincronizacion = "VERIFICADA", "PERFIL_PENDIENTE"
	a = &authIdentidadFalso{identidad: p}
	if r, err := verificarCon(a, u, "DOCUMENTO"); err != nil || r.Sincronizacion != "COMPLETA" || a.verificados != 0 || len(a.confirmados) != 1 {
		t.Fatalf("reanudar: %+v %v", r, err)
	}
}

func TestReconciliarIdentidades_CompletaCompensaYDejaPendiente(t *testing.T) {
	item := func(id string) *authclient.SolicitanteIdentity {
		return &authclient.SolicitanteIdentity{UserId: id, DniDeclarado: "1234567" + id[len(id)-1:], Estado: "VERIFICADA", Sincronizacion: "PERFIL_PENDIENTE"}
	}
	a := &authIdentidadFalso{pendientes: []*authclient.SolicitanteIdentity{item("u1"), item("u2"), item("u3")}, totalPend: 5}
	u := &usuariosIdentidadFalso{asignarErr: map[string]error{
		"u2": status.Error(codes.AlreadyExists, "identidad_conflicto: x"),
		"u3": status.Error(codes.Unavailable, "caído"),
	}}
	r, err := NewReconciliarIdentidadesLogic(context.Background(), &svc.ServiceContext{AuthClient: a, UsuariosClient: u}).
		ReconciliarIdentidades(&types.ReconciliarIdentidadesRequest{})
	if err != nil || r.Completadas != 1 || r.Revertidas != 1 || r.Pendientes != 3 {
		t.Fatalf("reconciliación: %+v %v", r, err)
	}
	if len(a.confirmados) != 1 || a.confirmados[0] != "u1:12345671" || len(a.revertidos) != 1 || a.revertidos[0] != "u2:12345672" ||
		len(a.fallos) != 1 || a.fallos[0] != "u3:usuarios_no_disponible" {
		t.Fatalf("operaciones: %+v", a)
	}
}
