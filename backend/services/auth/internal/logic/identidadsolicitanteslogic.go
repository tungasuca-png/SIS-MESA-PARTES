package logic

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"auth/auth"
	"auth/internal/interceptor"
	"auth/internal/repository"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Verificación de identidad de los solicitantes del registro público. Las
// cuatro RPC exigen el permiso solicitantes.identidad.verify (interceptor
// de autorización). Errores: "identidad_<clave>: <mensaje>". La auditoría
// guarda solo el evento y su resultado: nunca el DNI ni nombres.

var reUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func errIdentidad(c codes.Code, clave, mensaje string) error {
	return status.Error(c, "identidad_"+clave+": "+mensaje)
}

func usuarioIdentidad(id string) (string, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !reUUID.MatchString(id) {
		return "", errIdentidad(codes.InvalidArgument, "datos_invalidos", "El identificador del solicitante no es válido.")
	}
	return id, nil
}

func errDeIdentidad(ctx context.Context, err error) error {
	switch {
	case repository.EsquemaPendiente(err):
		return errIdentidad(codes.Unavailable, "no_disponible", "La verificación de identidad todavía no está habilitada.")
	case errors.Is(err, repository.ErrIdentidadSolicitanteNoEncontrada):
		return errIdentidad(codes.NotFound, "no_encontrada", "El solicitante no tiene una identidad declarada pendiente.")
	case errors.Is(err, repository.ErrIdentidadConflicto):
		return errIdentidad(codes.AlreadyExists, "conflicto",
			"El DNI declarado ya está asignado a otra cuenta. No se modificó ninguna cuenta.")
	case errors.Is(err, repository.ErrIdentidadDNIDistinto):
		return errIdentidad(codes.FailedPrecondition, "dni_distinto", "El DNI declarado cambió. Vuelve a consultar la solicitud.")
	case errors.Is(err, repository.ErrIdentidadYaVerificada):
		return errIdentidad(codes.FailedPrecondition, "ya_verificada", "La identidad ya fue verificada.")
	case errors.Is(err, repository.ErrIdentidadNoSincronizable):
		return errIdentidad(codes.FailedPrecondition, "no_sincronizable", "La identidad no tiene una sincronización pendiente con ese DNI.")
	case errors.Is(err, repository.ErrIdentidadNoSolicitante), errors.Is(err, repository.ErrRolNoPermitido):
		return errIdentidad(codes.FailedPrecondition, "no_solicitante", "La cuenta no es de un solicitante.")
	}
	logx.WithContext(ctx).Errorf("verificación de identidad: %v", err)
	return status.Error(codes.Internal, "no se pudo procesar la verificación de identidad")
}

func eventoIdentidad(ctx context.Context, svcCtx *svc.ServiceContext, usuarioID, nombre, resultado, motivo string) {
	if svcCtx.IdentidadRepository == nil {
		return
	}
	if err := svcCtx.IdentidadRepository.LogEvent(ctx, usuarioID, nombre, resultado, motivo); err != nil {
		logx.WithContext(ctx).Errorf("auditoría de identidad (%s): no se pudo registrar el evento", nombre)
	}
}

func motivoIdentidad(err error) string {
	msg := status.Convert(err).Message()
	if i := strings.Index(msg, ":"); strings.HasPrefix(msg, "identidad_") && i > 0 {
		return msg[len("identidad_"):i]
	}
	return status.Code(err).String()
}

func identidadProto(i *repository.IdentidadSolicitante) *auth.SolicitanteIdentity {
	p := &auth.SolicitanteIdentity{
		UserId: i.UsuarioID, DniDeclarado: i.DNIDeclarado, Origen: i.Origen, Estado: i.Estado,
		Metodo: i.Metodo, MotivoRechazo: i.MotivoRechazo, CreatedAt: i.CreatedAt.Format(time.RFC3339),
		Conflicto: i.Conflicto, Sincronizacion: i.Sincronizacion,
		SincronizacionIntentos: int32(i.SincronizacionIntentos), SincronizacionError: i.SincronizacionError,
	}
	if i.VerificadoAt != nil {
		p.VerificadoAt = i.VerificadoAt.Format(time.RFC3339)
	}
	return p
}

// ---- Listar ----

type ListSolicitanteIdentitiesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSolicitanteIdentitiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSolicitanteIdentitiesLogic {
	return &ListSolicitanteIdentitiesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListSolicitanteIdentitiesLogic) ListSolicitanteIdentities(in *auth.ListSolicitanteIdentitiesRequest) (*auth.ListSolicitanteIdentitiesResponse, error) {
	estado := strings.ToUpper(strings.TrimSpace(in.GetEstado()))
	switch estado {
	case "", repository.IdentidadPendiente, repository.IdentidadVerificada, repository.IdentidadRechazada:
	default:
		return nil, errIdentidad(codes.InvalidArgument, "datos_invalidos", "Estado no válido.")
	}
	limite, desplazamiento := int(in.GetLimit()), int(in.GetOffset())
	if limite <= 0 || limite > 100 {
		limite = 50
	}
	if desplazamiento < 0 {
		desplazamiento = 0
	}
	lista, total, err := l.svcCtx.IdentidadRepository.Listar(l.ctx, estado, in.GetSincronizacionPendiente(), limite, desplazamiento)
	if err != nil {
		return nil, errDeIdentidad(l.ctx, err)
	}
	resp := &auth.ListSolicitanteIdentitiesResponse{Total: int32(total)}
	for _, i := range lista {
		resp.Items = append(resp.Items, identidadProto(i))
	}
	return resp, nil
}

// ---- Consultar ----

type GetSolicitanteIdentityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSolicitanteIdentityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSolicitanteIdentityLogic {
	return &GetSolicitanteIdentityLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetSolicitanteIdentityLogic) GetSolicitanteIdentity(in *auth.GetSolicitanteIdentityRequest) (*auth.GetSolicitanteIdentityResponse, error) {
	id, err := usuarioIdentidad(in.GetUserId())
	if err != nil {
		return nil, err
	}
	i, err := l.svcCtx.IdentidadRepository.Obtener(l.ctx, id)
	if err != nil {
		return nil, errDeIdentidad(l.ctx, err)
	}
	return &auth.GetSolicitanteIdentityResponse{Identity: identidadProto(i)}, nil
}

// ---- Verificar ----

type VerifySolicitanteIdentityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifySolicitanteIdentityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifySolicitanteIdentityLogic {
	return &VerifySolicitanteIdentityLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// VerifySolicitanteIdentity escribe el DNI declarado en la cuenta (desde ahí
// también puede iniciar sesión con su DNI). Si el DNI ya pertenece a otra
// cuenta no cambia nada (conflicto, solo visible para el personal).
func (l *VerifySolicitanteIdentityLogic) VerifySolicitanteIdentity(in *auth.VerifySolicitanteIdentityRequest) (*auth.VerifySolicitanteIdentityResponse, error) {
	id, err := usuarioIdentidad(in.GetUserId())
	if err != nil {
		return nil, err
	}
	dni, err := normalizeDNI(in.GetDniEsperado())
	if err != nil {
		return nil, errIdentidad(codes.InvalidArgument, "datos_invalidos", "El DNI debe tener 8 dígitos numéricos.")
	}
	metodo := strings.ToUpper(strings.TrimSpace(in.GetMetodo()))
	if metodo != "PRESENCIAL" && metodo != "DOCUMENTO" {
		return nil, errIdentidad(codes.InvalidArgument, "datos_invalidos", "Indica cómo se cotejó la identidad: PRESENCIAL o DOCUMENTO.")
	}
	verificador, ok := interceptor.UserIDFromContext(l.ctx)
	if !ok || verificador == "" {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if verificador == id {
		return nil, errIdentidad(codes.PermissionDenied, "propia", "No puedes verificar tu propia identidad.")
	}
	ya, err := l.svcCtx.IdentidadRepository.Verificar(l.ctx, id, dni, metodo, verificador)
	if err != nil {
		e := errDeIdentidad(l.ctx, err)
		eventoIdentidad(l.ctx, l.svcCtx, id, "identidad_verificacion", "RECHAZADO", motivoIdentidad(e))
		return nil, e
	}
	if !ya {
		eventoIdentidad(l.ctx, l.svcCtx, id, "identidad_verificacion", "OK", strings.ToLower(metodo))
	}
	resp := &auth.VerifySolicitanteIdentityResponse{Estado: repository.IdentidadVerificada}
	if i, err := l.svcCtx.IdentidadRepository.Obtener(l.ctx, id); err == nil {
		resp.Sincronizacion = i.Sincronizacion
	}
	return resp, nil
}

// ---- Rechazar ----

type RejectSolicitanteIdentityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRejectSolicitanteIdentityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RejectSolicitanteIdentityLogic {
	return &RejectSolicitanteIdentityLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *RejectSolicitanteIdentityLogic) RejectSolicitanteIdentity(in *auth.RejectSolicitanteIdentityRequest) (*auth.RejectSolicitanteIdentityResponse, error) {
	id, err := usuarioIdentidad(in.GetUserId())
	if err != nil {
		return nil, err
	}
	motivo := espacios(in.GetMotivo())
	if len([]rune(motivo)) < 3 || len([]rune(motivo)) > 200 {
		return nil, errIdentidad(codes.InvalidArgument, "datos_invalidos", "Indica el motivo del rechazo (entre 3 y 200 caracteres).")
	}
	verificador, ok := interceptor.UserIDFromContext(l.ctx)
	if !ok || verificador == "" {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if verificador == id {
		return nil, errIdentidad(codes.PermissionDenied, "propia", "No puedes resolver tu propia identidad.")
	}
	if err := l.svcCtx.IdentidadRepository.Rechazar(l.ctx, id, motivo, verificador); err != nil {
		e := errDeIdentidad(l.ctx, err)
		eventoIdentidad(l.ctx, l.svcCtx, id, "identidad_rechazo", "RECHAZADO", motivoIdentidad(e))
		return nil, e
	}
	eventoIdentidad(l.ctx, l.svcCtx, id, "identidad_rechazo", "OK", "")
	return &auth.RejectSolicitanteIdentityResponse{Estado: repository.IdentidadRechazada}, nil
}

// ---- Sincronización con Usuarios Service (orquestada por el Gateway) ----
//
// Auth guarda el estado de la operación: VerifySolicitanteIdentity deja la
// sincronización en PERFIL_PENDIENTE; el Gateway asigna el DNI en Usuarios y
// luego llama a Confirm (COMPLETA), a RecordFailure (sigue pendiente, con la
// traza) o a Revert (Usuarios encontró el DNI en OTRO perfil: compensación).

var reCodigoSincronizacion = regexp.MustCompile(`^[a-z_]{1,60}$`)

func sincronizacionDe(ctx context.Context, svcCtx *svc.ServiceContext, id string) *auth.IdentitySyncResponse {
	resp := &auth.IdentitySyncResponse{}
	if i, err := svcCtx.IdentidadRepository.Obtener(ctx, id); err == nil {
		resp.Estado, resp.Sincronizacion = i.Estado, i.Sincronizacion
	}
	return resp
}

func datosSincronizacion(in *auth.IdentitySyncRequest) (string, string, error) {
	id, err := usuarioIdentidad(in.GetUserId())
	if err != nil {
		return "", "", err
	}
	dni, err := normalizeDNI(in.GetDni())
	if err != nil {
		return "", "", errIdentidad(codes.InvalidArgument, "datos_invalidos", "El DNI debe tener 8 dígitos numéricos.")
	}
	return id, dni, nil
}

type ConfirmSolicitanteIdentitySyncLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfirmSolicitanteIdentitySyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmSolicitanteIdentitySyncLogic {
	return &ConfirmSolicitanteIdentitySyncLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// ConfirmSolicitanteIdentitySync: Usuarios ya tiene el DNI. Idempotente.
func (l *ConfirmSolicitanteIdentitySyncLogic) ConfirmSolicitanteIdentitySync(in *auth.IdentitySyncRequest) (*auth.IdentitySyncResponse, error) {
	id, dni, err := datosSincronizacion(in)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.IdentidadRepository.ConfirmarSincronizacion(l.ctx, id, dni); err != nil {
		return nil, errDeIdentidad(l.ctx, err)
	}
	eventoIdentidad(l.ctx, l.svcCtx, id, "identidad_sincronizacion", "OK", "")
	return sincronizacionDe(l.ctx, l.svcCtx, id), nil
}

type RecordSolicitanteIdentitySyncFailureLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRecordSolicitanteIdentitySyncFailureLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RecordSolicitanteIdentitySyncFailureLogic {
	return &RecordSolicitanteIdentitySyncFailureLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// RecordSolicitanteIdentitySyncFailure: un intento de sincronización falló;
// la operación queda PERFIL_PENDIENTE para la reconciliación.
func (l *RecordSolicitanteIdentitySyncFailureLogic) RecordSolicitanteIdentitySyncFailure(in *auth.IdentitySyncFailureRequest) (*auth.IdentitySyncResponse, error) {
	id, err := usuarioIdentidad(in.GetUserId())
	if err != nil {
		return nil, err
	}
	codigo := strings.TrimSpace(in.GetCodigo())
	if !reCodigoSincronizacion.MatchString(codigo) {
		return nil, errIdentidad(codes.InvalidArgument, "datos_invalidos", "Código de error no válido.")
	}
	if err := l.svcCtx.IdentidadRepository.RegistrarFalloSincronizacion(l.ctx, id, codigo); err != nil {
		return nil, errDeIdentidad(l.ctx, err)
	}
	eventoIdentidad(l.ctx, l.svcCtx, id, "identidad_sincronizacion", "ERROR", codigo)
	return sincronizacionDe(l.ctx, l.svcCtx, id), nil
}

type RevertSolicitanteIdentityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRevertSolicitanteIdentityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevertSolicitanteIdentityLogic {
	return &RevertSolicitanteIdentityLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// RevertSolicitanteIdentity: compensación. Usuarios encontró el DNI en OTRO
// perfil: Auth quita el DNI de esta cuenta y la identidad vuelve a PENDIENTE
// con el conflicto visible. No elimina ni recrea la cuenta.
func (l *RevertSolicitanteIdentityLogic) RevertSolicitanteIdentity(in *auth.IdentitySyncRequest) (*auth.IdentitySyncResponse, error) {
	id, dni, err := datosSincronizacion(in)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.IdentidadRepository.RevertirVerificacion(l.ctx, id, dni); err != nil {
		e := errDeIdentidad(l.ctx, err)
		eventoIdentidad(l.ctx, l.svcCtx, id, "identidad_reversion", "RECHAZADO", motivoIdentidad(e))
		return nil, e
	}
	eventoIdentidad(l.ctx, l.svcCtx, id, "identidad_reversion", "OK", repository.ErrorConflictoPerfil)
	return sincronizacionDe(l.ctx, l.svcCtx, id), nil
}
