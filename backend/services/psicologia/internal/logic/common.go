package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"psicologia/internal/authorization"
	"psicologia/internal/interceptor"
	"psicologia/internal/repository"
	"psicologia/internal/svc"
	"psicologia/internal/validation"
	"psicologia/psicologia"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"usuarios/usuariosclient"

	"google.golang.org/grpc/metadata"
)

// Permisos del módulo usados en la FASE 1. El servicio nunca decide por el
// nombre del rol: solo pregunta a Auth.HasPermission con la identidad del JWT.
const (
	permExpedientesView    = "psicologia.expedientes.view"
	permExpedientesViewOwn = "psicologia.expedientes.view_own"
	permExpedientesCreate  = "psicologia.expedientes.create"
	permExpedientesUpdate  = "psicologia.expedientes.update"
)

// Códigos de motivo de auditoría (texto corto del sistema, nunca del usuario).
const (
	motivoPermisoInsuficiente = "PERMISO_INSUFICIENTE"
	motivoSinAlcance          = "SIN_ALCANCE"
	motivoFueraDeAlcance      = "FUERA_DE_ALCANCE"
	motivoListado             = "LISTADO"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

const msgSinPermiso = "el usuario no tiene el permiso requerido"

// authenticatedUser devuelve el usuario autenticado por el interceptor. El
// userID sale SIEMPRE del JWT, nunca de la solicitud.
func authenticatedUser(ctx context.Context) (string, error) {
	userID, _, ok := interceptor.UserFromContext(ctx)
	if !ok || !validation.IsValidUUID(userID) {
		return "", status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	return userID, nil
}

// scope es el ALCANCE DE DATOS resuelto a partir de permisos (no de roles):
//
//	psicologia.expedientes.view      -> all: cualquier expediente
//	psicologia.expedientes.view_own  -> solo expedientes con responsable_id == userID
//
// El scope se resuelve una sola vez, en el expediente; los recursos hijos de
// fases futuras heredarán este mismo alcance.
type scope struct{ all bool }

// resolveScope consulta Auth. ok=false significa que el usuario no tiene
// NINGÚN permiso de lectura de expedientes. Cualquier error de Auth se
// propaga (fail-closed).
func resolveScope(ctx context.Context, sc *svc.ServiceContext, userID string) (s scope, ok bool, err error) {
	hasView, err := authorization.HasPermission(ctx, sc.AuthClient, userID, permExpedientesView)
	if err != nil {
		return scope{}, false, err
	}
	if hasView {
		return scope{all: true}, true, nil
	}
	hasOwn, err := authorization.HasPermission(ctx, sc.AuthClient, userID, permExpedientesViewOwn)
	if err != nil {
		return scope{}, false, err
	}
	return scope{}, hasOwn, nil
}

// responsableFilter es el filtro SQL de alcance: vacío = sin restricción.
func (s scope) responsableFilter(userID string) string {
	if s.all {
		return ""
	}
	return userID
}

func (s scope) allows(e *repository.Expediente, userID string) bool {
	return s.all || e.ResponsableID == userID
}

// requirePermission exige un permiso y, si falta, registra el evento DENIED.
// Un error de Auth se propaga sin auditar (no es una denegación).
func requirePermission(ctx context.Context, sc *svc.ServiceContext, userID, permission, recursoID string) error {
	allowed, err := authorization.HasPermission(ctx, sc.AuthClient, userID, permission)
	if err != nil {
		return err
	}
	if !allowed {
		return denied(ctx, sc, userID, recursoID, "", motivoPermisoInsuficiente)
	}
	return nil
}

// denied registra DENIED (mejor esfuerzo: si la bitácora falla, igual se
// deniega) y devuelve PermissionDenied sin revelar nada del recurso.
func denied(ctx context.Context, sc *svc.ServiceContext, userID, recursoID, estudianteID, motivo string) error {
	auditDenied(ctx, sc, userID, recursoID, estudianteID, motivo)
	return status.Error(codes.PermissionDenied, msgSinPermiso)
}

func auditDenied(ctx context.Context, sc *svc.ServiceContext, userID, recursoID, estudianteID, motivo string) {
	if err := sc.AuditoriaRepo.Registrar(ctx, repository.AuditEntry{
		UsuarioID: userID, Accion: repository.AccionDenied, Recurso: repository.RecursoExpediente,
		RecursoID: recursoID, EstudianteID: estudianteID, Resultado: repository.ResultadoDenegado, Motivo: motivo,
	}); err != nil {
		logx.WithContext(ctx).Errorf("no se pudo registrar el evento DENIED en la auditoría: %v", err)
	}
}

// notFoundHidden: un expediente fuera del alcance del usuario responde igual
// que uno inexistente (no se revela su existencia), pero el intento queda en
// la bitácora como DENIED.
func notFoundHidden(ctx context.Context, sc *svc.ServiceContext, userID, recursoID, estudianteID string) error {
	auditDenied(ctx, sc, userID, recursoID, estudianteID, motivoFueraDeAlcance)
	return status.Error(codes.NotFound, msgExpedienteNoEncontrado)
}

const msgExpedienteNoEncontrado = "expediente psicológico no encontrado"

// auditView registra la lectura ANTES de devolver el dato. Es fail-closed: si
// la bitácora no se puede escribir, no se entrega información sensible.
func auditView(ctx context.Context, sc *svc.ServiceContext, userID, recursoID, estudianteID, motivo string) error {
	if err := sc.AuditoriaRepo.Registrar(ctx, repository.AuditEntry{
		UsuarioID: userID, Accion: repository.AccionView, Recurso: repository.RecursoExpediente,
		RecursoID: recursoID, EstudianteID: estudianteID, Resultado: repository.ResultadoPermitido, Motivo: motivo,
	}); err != nil {
		logx.WithContext(ctx).Errorf("no se pudo registrar el evento VIEW en la auditoría: %v", err)
		return status.Error(codes.Internal, "no se pudo registrar la auditoría de la consulta")
	}
	return nil
}

// repoError traduce errores del repositorio a códigos gRPC sin exponer texto
// de PostgreSQL. Los errores desconocidos se registran (sin contenido clínico)
// y se responden como Internal genérico.
func repoError(ctx context.Context, err error, internalMsg string) error {
	switch {
	case errors.Is(err, repository.ErrExpedienteNotFound):
		return status.Error(codes.NotFound, msgExpedienteNoEncontrado)
	case errors.Is(err, repository.ErrEstudianteYaTieneExpediente):
		return status.Error(codes.AlreadyExists, "el estudiante ya tiene un expediente psicológico")
	case errors.Is(err, repository.ErrEstadoInvalido):
		return status.Error(codes.FailedPrecondition, "el estado actual del expediente no permite esta operación")
	}
	logx.WithContext(ctx).Errorf("%s: %v", internalMsg, err)
	return status.Error(codes.Internal, internalMsg)
}

// tipoUsuarioPsicologo es el valor real de Usuarios.UsuarioBasic.TipoUsuario
// para el rol PSICOLOGO (mismo catálogo que usuarios.tipo_usuario).
const tipoUsuarioPsicologo = "PSICOLOGO"

// validateResponsable comprueba que responsable_id corresponde a un usuario
// real (Usuarios.GetUsuariosBasic, reenviando el JWT del solicitante) CUYO
// tipo_usuario sea PSICOLOGO. El usuario autenticado siempre es válido como
// responsable de sí mismo (ya demostró, más arriba, tener un permiso de
// Psicología; no hace falta consultar su tipo_usuario).
//
// LIMITACIÓN (documentada): Auth.HasPermission solo responde por el usuario
// autenticado, así que no se puede verificar que UN TERCERO tenga realmente
// asignado un permiso de Psicología (p. ej. si a su rol PSICOLOGO le
// retiraron los permisos desde Roles y Permisos). Exigir tipo_usuario =
// PSICOLOGO es la validación de dominio disponible sin ese RPC. Fail-closed
// si Usuarios no responde.
func validateResponsable(ctx context.Context, sc *svc.ServiceContext, userID, responsableID string) error {
	if !validation.IsValidUUID(responsableID) {
		return status.Error(codes.InvalidArgument, "el responsable_id no es válido")
	}
	if strings.EqualFold(responsableID, userID) {
		return nil
	}
	outCtx := ctx
	if token, ok := interceptor.TokenFromContext(ctx); ok {
		outCtx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	resp, err := sc.UsuariosClient.GetUsuariosBasic(outCtx, &usuariosclient.GetUsuariosBasicRequest{Ids: []string{responsableID}})
	if err != nil {
		return err
	}
	if resp == nil || len(resp.Usuarios) == 0 {
		return status.Error(codes.InvalidArgument, "el responsable indicado no corresponde a un usuario registrado")
	}
	if resp.Usuarios[0].TipoUsuario != tipoUsuarioPsicologo {
		return status.Error(codes.InvalidArgument, "el responsable indicado no corresponde a un psicólogo registrado")
	}
	return nil
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// toProto convierte el modelo al mensaje. includeResumen=false en listados:
// resumen_caso y motivo_cierre son texto libre sensible y solo se devuelven en
// el detalle (FASE 6N-3, bloque 13).
// accesoClinico indica si el usuario puede ver el CONTENIDO clínico del
// expediente (resumen del caso y motivo de cierre). El alcance del
// expediente (view/view_own) solo da acceso a sus datos administrativos; el
// contenido clínico exige además "psicologia.atenciones.view", el mismo
// permiso que protege las atenciones. Un error de Auth se propaga
// (fail-closed: no se entrega nada).
func accesoClinico(ctx context.Context, sc *svc.ServiceContext, userID string) (bool, error) {
	return authorization.HasPermission(ctx, sc.AuthClient, userID, permAtencionesView)
}

func toProto(e *repository.Expediente, includeResumen bool) *psicologia.ExpedientePsicologico {
	out := &psicologia.ExpedientePsicologico{
		Id:            e.ID,
		EstudianteId:  e.EstudianteID,
		Codigo:        e.Codigo,
		Estado:        e.Estado,
		ResponsableId: e.ResponsableID,
		AbiertoPor:    e.AbiertoPor,
		FechaApertura: formatTime(e.FechaApertura),
		CreatedAt:     formatTime(e.CreatedAt),
		UpdatedAt:     formatTime(e.UpdatedAt),
	}
	if e.FechaCierre != nil {
		out.FechaCierre = formatTime(*e.FechaCierre)
	}
	if includeResumen && e.MotivoCierre != nil {
		out.MotivoCierre = *e.MotivoCierre
	}
	if includeResumen && e.ResumenCaso != nil {
		out.ResumenCaso = *e.ResumenCaso
	}
	return out
}

// loadInScope carga un expediente para una mutación aplicando el alcance del
// usuario: sin permiso de lectura -> PermissionDenied (DENIED); fuera de su
// alcance -> NotFound uniforme (DENIED). Devuelve el filtro de alcance que el
// repositorio vuelve a aplicar dentro de su transacción.
func loadInScope(ctx context.Context, sc *svc.ServiceContext, userID, id string) (*repository.Expediente, string, error) {
	s, ok, err := resolveScope(ctx, sc, userID)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", denied(ctx, sc, userID, id, "", motivoSinAlcance)
	}
	exp, err := sc.ExpedienteRepo.GetByID(ctx, id)
	if err != nil {
		return nil, "", repoError(ctx, err, "no se pudo consultar el expediente psicológico")
	}
	if !s.allows(exp, userID) {
		return nil, "", notFoundHidden(ctx, sc, userID, id, exp.EstudianteID)
	}
	return exp, s.responsableFilter(userID), nil
}
