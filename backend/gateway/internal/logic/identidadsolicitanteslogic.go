package logic

import (
	"context"
	"strings"

	"gateway/internal/svc"
	"gateway/internal/types"

	"auth/authclient"
	"usuarios/usuariosclient"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Verificación de identidad de solicitantes (bandeja de Secretaría). El
// permiso solicitantes.identidad.verify lo comprueban Auth (interceptor) y
// Usuarios (lógica) con el token del personal: el Gateway solo reenvía.

// ---- Listar ----

type ListIdentidadesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListIdentidadesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListIdentidadesLogic {
	return &ListIdentidadesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListIdentidadesLogic) ListIdentidades(req *types.ListIdentidadesRequest) (*types.ListIdentidadesResponse, error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	r, err := l.svcCtx.AuthClient.ListSolicitanteIdentities(ctx, &authclient.ListSolicitanteIdentitiesRequest{
		Estado: req.Estado, Limit: req.Limit, Offset: req.Offset, SincronizacionPendiente: req.SincronizacionPendiente,
	})
	if err != nil {
		return nil, err
	}
	nombres := map[string]string{}
	if len(r.GetItems()) > 0 {
		ids := make([]string, 0, len(r.GetItems()))
		for _, i := range r.GetItems() {
			ids = append(ids, i.GetUserId())
		}
		// Sin nombres si Usuarios no responde: la bandeja sigue sirviendo.
		if b, err := l.svcCtx.UsuariosClient.GetUsuariosBasic(ctx, &usuariosclient.GetUsuariosBasicRequest{Ids: ids}); err == nil {
			for _, u := range b.GetUsuarios() {
				nombres[u.GetId()] = u.GetNombreCompleto()
			}
		} else {
			l.Errorf("identidades: no se pudieron resolver los nombres (%v)", status.Code(err))
		}
	}
	resp := &types.ListIdentidadesResponse{Items: []types.IdentidadSolicitanteDTO{}, Total: r.GetTotal()}
	for _, i := range r.GetItems() {
		resp.Items = append(resp.Items, types.IdentidadSolicitanteDTO{
			UsuarioId: i.GetUserId(), NombreCompleto: nombres[i.GetUserId()], DniDeclarado: i.GetDniDeclarado(),
			Origen: i.GetOrigen(), Estado: i.GetEstado(), Metodo: i.GetMetodo(), VerificadoAt: i.GetVerificadoAt(),
			MotivoRechazo: i.GetMotivoRechazo(), CreatedAt: i.GetCreatedAt(), Conflicto: i.GetConflicto(),
			Sincronizacion: i.GetSincronizacion(), SincronizacionIntentos: i.GetSincronizacionIntentos(),
			SincronizacionError: i.GetSincronizacionError(),
		})
	}
	return resp, nil
}

// ---- Verificar y sincronizar ----

// Auth y Usuarios tienen bases separadas: no hay transacción común. Auth es
// la fuente de verdad y guarda el estado de la operación
// (identidad_solicitantes.sincronizacion, migración 025):
//  1. Auth verifica en una transacción (DNI definitivo + VERIFICADA +
//     PERFIL_PENDIENTE). Si el DNI ya es de otra cuenta: conflicto y nada
//     cambia en ningún servicio.
//  2. Usuarios pasa el DNI declarado al perfil (idempotente).
//     - Bien: Auth confirma (COMPLETA).
//     - DNI en OTRO perfil: Auth revierte su verificación (compensación) y
//       la identidad vuelve a PENDIENTE con el conflicto visible.
//     - Usuarios no disponible u otro fallo: queda PERFIL_PENDIENTE con la
//       traza del intento; la reconciliación lo reintenta.
//  3. Si la compensación también falla, sigue PERFIL_PENDIENTE: la
//     reconciliación vuelve a intentar Usuarios y, ante el mismo conflicto,
//     la compensación. Nunca se elimina ni se recrea una cuenta.

const (
	identidadVerificada            = "VERIFICADA"
	sincronizacionCompleta         = "COMPLETA"
	sincronizacionPerfilPendiente  = "PERFIL_PENDIENTE"
	mensajeConflictoIdentidad      = "identidad_conflicto: El DNI declarado ya está asignado a otra cuenta. No se modificó ninguna cuenta."
	mensajeConflictoReconciliacion = "identidad_conflicto: El DNI declarado ya está asignado a otro perfil. La verificación quedó pendiente de reconciliación y se revertirá automáticamente."
)

var errConflictoIdentidad = status.Error(codes.AlreadyExists, mensajeConflictoIdentidad)

// codigoFalloUsuarios: código estable del fallo (traza en Auth, sin datos).
func codigoFalloUsuarios(err error) string {
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		return "usuarios_no_disponible"
	case codes.NotFound:
		return "usuarios_sin_perfil"
	case codes.FailedPrecondition:
		return "usuarios_no_asignable"
	case codes.PermissionDenied, codes.Unauthenticated:
		return "usuarios_sin_permiso"
	}
	return "usuarios_error"
}

// sincronizarPerfil ejecuta el paso 2 para una identidad ya VERIFICADA en
// Auth. Devuelve la sincronización resultante; error solo ante un conflicto
// (la identidad quedó revertida, o pendiente de revertir).
func sincronizarPerfil(ctx context.Context, svcCtx *svc.ServiceContext, log logx.Logger, usuarioID, dni string) (string, error) {
	sync := &authclient.IdentitySyncRequest{UserId: usuarioID, Dni: dni}
	_, err := svcCtx.UsuariosClient.AssignVerifiedDni(ctx, &usuariosclient.VerifiedDniRequest{Id: usuarioID, Dni: dni})
	switch status.Code(err) {
	case codes.OK:
		if _, cerr := svcCtx.AuthClient.ConfirmSolicitanteIdentitySync(ctx, sync); cerr != nil {
			// El perfil ya tiene el DNI; la próxima reconciliación confirma.
			log.Errorf("identidad: perfil actualizado pero Auth no confirmó (%v); queda para la reconciliación", status.Code(cerr))
			return sincronizacionPerfilPendiente, nil
		}
		return sincronizacionCompleta, nil
	case codes.AlreadyExists:
		if _, rerr := svcCtx.AuthClient.RevertSolicitanteIdentity(ctx, sync); rerr != nil {
			log.Errorf("identidad: conflicto en el perfil y la compensación falló (%v); queda para la reconciliación", status.Code(rerr))
			if _, ferr := svcCtx.AuthClient.RecordSolicitanteIdentitySyncFailure(ctx, &authclient.IdentitySyncFailureRequest{
				UserId: usuarioID, Codigo: "conflicto_perfil_reversion_pendiente",
			}); ferr != nil {
				log.Errorf("identidad: tampoco se pudo registrar el fallo (%v)", status.Code(ferr))
			}
			return sincronizacionPerfilPendiente, status.Error(codes.AlreadyExists, mensajeConflictoReconciliacion)
		}
		return "", errConflictoIdentidad
	}
	codigo := codigoFalloUsuarios(err)
	log.Errorf("identidad: no se pudo actualizar el perfil (%s); queda para la reconciliación", codigo)
	if _, ferr := svcCtx.AuthClient.RecordSolicitanteIdentitySyncFailure(ctx, &authclient.IdentitySyncFailureRequest{
		UserId: usuarioID, Codigo: codigo,
	}); ferr != nil {
		log.Errorf("identidad: no se pudo registrar el fallo de sincronización (%v)", status.Code(ferr))
	}
	return sincronizacionPerfilPendiente, nil
}

type VerificarIdentidadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewVerificarIdentidadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerificarIdentidadLogic {
	return &VerificarIdentidadLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// VerificarIdentidad: repetirla es seguro (una verificación completa no hace
// nada; una con el perfil pendiente reanuda la sincronización).
func (l *VerificarIdentidadLogic) VerificarIdentidad(req *types.VerificarIdentidadRequest) (*types.IdentidadEstadoResponse, error) {
	metodo := strings.ToUpper(strings.TrimSpace(req.Metodo))
	if metodo != "PRESENCIAL" && metodo != "DOCUMENTO" {
		return nil, status.Error(codes.InvalidArgument, "identidad_datos_invalidos: Indica cómo se cotejó la identidad: PRESENCIAL o DOCUMENTO.")
	}
	ctx := withAuthorization(l.ctx, req.Authorization)
	g, err := l.svcCtx.AuthClient.GetSolicitanteIdentity(ctx, &authclient.GetSolicitanteIdentityRequest{UserId: req.Id})
	if err != nil {
		return nil, err
	}
	ident := g.GetIdentity()
	if ident.GetEstado() == identidadVerificada {
		if ident.GetSincronizacion() == sincronizacionCompleta {
			return &types.IdentidadEstadoResponse{Estado: identidadVerificada, Sincronizacion: sincronizacionCompleta}, nil
		}
	} else {
		if ident.GetConflicto() {
			return nil, errConflictoIdentidad
		}
		if _, err := l.svcCtx.AuthClient.VerifySolicitanteIdentity(ctx, &authclient.VerifySolicitanteIdentityRequest{
			UserId: ident.GetUserId(), DniEsperado: ident.GetDniDeclarado(), Metodo: metodo,
		}); err != nil {
			return nil, err // Auth no cambió nada (conflicto, permiso o datos)
		}
	}
	sincronizacion, err := sincronizarPerfil(ctx, l.svcCtx, l.Logger, ident.GetUserId(), ident.GetDniDeclarado())
	if err != nil {
		return nil, err
	}
	return &types.IdentidadEstadoResponse{Estado: identidadVerificada, Sincronizacion: sincronizacion}, nil
}

// ---- Reconciliar ----

type ReconciliarIdentidadesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReconciliarIdentidadesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReconciliarIdentidadesLogic {
	return &ReconciliarIdentidadesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// ReconciliarIdentidades reintenta, con la sesión del personal autorizado,
// las verificaciones cuyo perfil quedó pendiente (hasta 100 por llamada; la
// respuesta dice cuántas siguen pendientes).
func (l *ReconciliarIdentidadesLogic) ReconciliarIdentidades(req *types.ReconciliarIdentidadesRequest) (*types.ReconciliarIdentidadesResponse, error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	r, err := l.svcCtx.AuthClient.ListSolicitanteIdentities(ctx, &authclient.ListSolicitanteIdentitiesRequest{
		Estado: identidadVerificada, SincronizacionPendiente: true, Limit: 100,
	})
	if err != nil {
		return nil, err
	}
	resp := &types.ReconciliarIdentidadesResponse{}
	for _, i := range r.GetItems() {
		s, err := sincronizarPerfil(ctx, l.svcCtx, l.Logger, i.GetUserId(), i.GetDniDeclarado())
		switch {
		case err == nil && s == sincronizacionCompleta:
			resp.Completadas++
		case err != nil && s == "":
			resp.Revertidas++
		default:
			resp.Pendientes++
		}
	}
	resp.Pendientes += int(r.GetTotal()) - len(r.GetItems())
	return resp, nil
}

// ---- Rechazar ----

type RechazarIdentidadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRechazarIdentidadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RechazarIdentidadLogic {
	return &RechazarIdentidadLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RechazarIdentidadLogic) RechazarIdentidad(req *types.RechazarIdentidadRequest) (*types.IdentidadEstadoResponse, error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	r, err := l.svcCtx.AuthClient.RejectSolicitanteIdentity(ctx, &authclient.RejectSolicitanteIdentityRequest{UserId: req.Id, Motivo: req.Motivo})
	if err != nil {
		return nil, err
	}
	return &types.IdentidadEstadoResponse{Estado: r.GetEstado()}, nil
}
