package logic

import (
	"context"
	"strings"

	"psicologia/internal/repository"
	"psicologia/internal/svc"
	"psicologia/internal/validation"
	"psicologia/psicologia"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Las mutaciones exigen (1) el permiso de escritura correspondiente y (2)
// estar dentro del alcance del usuario (view / view_own). El evento de
// auditoría se escribe en la MISMA transacción que el cambio (ver repository).

// ---- OpenExpediente ----

type OpenExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOpenExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenExpedienteLogic {
	return &OpenExpedienteLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *OpenExpedienteLogic) OpenExpediente(in *psicologia.OpenExpedienteRequest) (*psicologia.OpenExpedienteResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := requirePermission(l.ctx, l.svcCtx, userID, permExpedientesCreate, ""); err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "la solicitud es obligatoria")
	}

	estudianteID := strings.TrimSpace(in.EstudianteId)
	if !validation.IsValidUUID(estudianteID) {
		return nil, status.Error(codes.InvalidArgument, "el estudiante_id no es válido")
	}
	if !validation.TextoDentroDeLimite(in.ResumenCaso, validation.MaxResumenCaso) {
		return nil, status.Error(codes.InvalidArgument, "el resumen del caso es demasiado largo")
	}

	// FASE 5C: si hay verificador configurado, se exige que el estudiante
	// exista realmente en Estudiantes Service (nil solo en tests que
	// deliberadamente prueban el comportamiento sin esa dependencia — ver
	// svc.EstudianteVerifier). Cualquier error de la verificación (servicio
	// caído, permiso rechazado, JWT inválido, error interno) se propaga TAL
	// CUAL — fail-closed: nunca se traduce en "el estudiante no existe", que
	// solo corresponde al caso donde Estudiantes Service confirmó, sin error,
	// que ese id no está.
	if verifier := l.svcCtx.Estudiantes; verifier != nil {
		exists, err := verifier.Exists(l.ctx, estudianteID)
		if err != nil {
			l.Errorf("no se pudo verificar al estudiante: %v", err)
			return nil, err
		}
		if !exists {
			return nil, status.Error(codes.NotFound, "el estudiante no existe")
		}
	}

	// abierto_por SIEMPRE es el usuario autenticado. El responsable, si no se
	// indica, es quien abre; si se indica otro, se valida contra Usuarios.
	responsableID := userID
	if requested := strings.TrimSpace(in.ResponsableId); requested != "" {
		if err := validateResponsable(l.ctx, l.svcCtx, userID, requested); err != nil {
			return nil, err
		}
		responsableID = strings.ToLower(requested)
	}

	exp, err := l.svcCtx.ExpedienteRepo.Open(l.ctx, repository.OpenInput{
		EstudianteID:  strings.ToLower(estudianteID),
		ResponsableID: responsableID,
		AbiertoPor:    userID,
		ResumenCaso:   in.ResumenCaso,
	})
	if err != nil {
		return nil, repoError(l.ctx, err, "no se pudo abrir el expediente psicológico")
	}
	return &psicologia.OpenExpedienteResponse{Expediente: toProto(exp, true)}, nil
}

// ---- UpdateExpediente ----

type UpdateExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateExpedienteLogic {
	return &UpdateExpedienteLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateExpedienteLogic) UpdateExpediente(in *psicologia.UpdateExpedienteRequest) (*psicologia.UpdateExpedienteResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	id := ""
	if in != nil {
		id = strings.TrimSpace(in.Id)
	}
	if err := requirePermission(l.ctx, l.svcCtx, userID, permExpedientesUpdate, id); err != nil {
		return nil, err
	}
	if !validation.IsValidUUID(id) {
		return nil, status.Error(codes.InvalidArgument, "el identificador no es válido")
	}

	var campos []string
	update := repository.UpdateInput{ID: id, ActorID: userID}
	if in.ResumenCaso != nil {
		if !validation.TextoDentroDeLimite(*in.ResumenCaso, validation.MaxResumenCaso) {
			return nil, status.Error(codes.InvalidArgument, "el resumen del caso es demasiado largo")
		}
		update.ResumenCaso = in.ResumenCaso
		campos = append(campos, "resumen_caso")
	}
	if in.Estado != nil {
		estado := strings.ToUpper(strings.TrimSpace(*in.Estado))
		if !validation.IsEstadoActivo(estado) {
			return nil, status.Error(codes.InvalidArgument, "el estado debe ser ABIERTO o EN_SEGUIMIENTO (para cerrar use el cierre)")
		}
		update.Estado = &estado
		campos = append(campos, "estado")
	}
	if in.ResponsableId != nil {
		responsable := strings.ToLower(strings.TrimSpace(*in.ResponsableId))
		if !validation.IsValidUUID(responsable) {
			return nil, status.Error(codes.InvalidArgument, "el responsable_id no es válido")
		}
		update.ResponsableID = &responsable
		campos = append(campos, "responsable_id")
	}
	if len(campos) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no se indicó ningún campo para modificar")
	}
	update.Campos = strings.Join(campos, ",")

	exp, scopeFilter, err := loadInScope(l.ctx, l.svcCtx, userID, id)
	if err != nil {
		return nil, err
	}
	if update.ResponsableID != nil && *update.ResponsableID != exp.ResponsableID {
		if err := validateResponsable(l.ctx, l.svcCtx, userID, *update.ResponsableID); err != nil {
			return nil, err
		}
	}
	update.ScopeResponsableID = scopeFilter

	updated, err := l.svcCtx.ExpedienteRepo.Update(l.ctx, update)
	if err != nil {
		return nil, repoError(l.ctx, err, "no se pudo actualizar el expediente psicológico")
	}
	return &psicologia.UpdateExpedienteResponse{Expediente: toProto(updated, true)}, nil
}

// ---- CloseExpediente ----

type CloseExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseExpedienteLogic {
	return &CloseExpedienteLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// CloseExpediente: ABIERTO/EN_SEGUIMIENTO -> CERRADO, con motivo obligatorio.
func (l *CloseExpedienteLogic) CloseExpediente(in *psicologia.CloseExpedienteRequest) (*psicologia.CloseExpedienteResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	id, motivo := "", ""
	if in != nil {
		id, motivo = strings.TrimSpace(in.Id), strings.TrimSpace(in.Motivo)
	}
	if err := requirePermission(l.ctx, l.svcCtx, userID, permExpedientesUpdate, id); err != nil {
		return nil, err
	}
	if !validation.IsValidUUID(id) {
		return nil, status.Error(codes.InvalidArgument, "el identificador no es válido")
	}
	if motivo == "" {
		return nil, status.Error(codes.InvalidArgument, "el motivo de cierre es obligatorio")
	}
	if !validation.TextoDentroDeLimite(motivo, validation.MaxMotivo) {
		return nil, status.Error(codes.InvalidArgument, "el motivo de cierre es demasiado largo")
	}

	_, scopeFilter, err := loadInScope(l.ctx, l.svcCtx, userID, id)
	if err != nil {
		return nil, err
	}
	closed, err := l.svcCtx.ExpedienteRepo.Close(l.ctx, id, userID, scopeFilter, motivo)
	if err != nil {
		return nil, repoError(l.ctx, err, "no se pudo cerrar el expediente psicológico")
	}
	return &psicologia.CloseExpedienteResponse{Expediente: toProto(closed, true)}, nil
}

// ---- ReopenExpediente ----

type ReopenExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReopenExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReopenExpedienteLogic {
	return &ReopenExpedienteLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// ReopenExpediente: ÚNICA transición permitida desde CERRADO: a ABIERTO (por
// defecto) o a EN_SEGUIMIENTO. No hay otras transiciones.
func (l *ReopenExpedienteLogic) ReopenExpediente(in *psicologia.ReopenExpedienteRequest) (*psicologia.ReopenExpedienteResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	id, estado := "", ""
	if in != nil {
		id, estado = strings.TrimSpace(in.Id), strings.ToUpper(strings.TrimSpace(in.Estado))
	}
	if err := requirePermission(l.ctx, l.svcCtx, userID, permExpedientesUpdate, id); err != nil {
		return nil, err
	}
	if !validation.IsValidUUID(id) {
		return nil, status.Error(codes.InvalidArgument, "el identificador no es válido")
	}
	if estado == "" {
		estado = validation.EstadoAbierto
	}
	if !validation.IsEstadoActivo(estado) {
		return nil, status.Error(codes.InvalidArgument, "un expediente cerrado solo puede reabrirse como ABIERTO o EN_SEGUIMIENTO")
	}

	_, scopeFilter, err := loadInScope(l.ctx, l.svcCtx, userID, id)
	if err != nil {
		return nil, err
	}
	reopened, err := l.svcCtx.ExpedienteRepo.Reopen(l.ctx, id, userID, scopeFilter, estado)
	if err != nil {
		return nil, repoError(l.ctx, err, "no se pudo reabrir el expediente psicológico")
	}
	return &psicologia.ReopenExpedienteResponse{Expediente: toProto(reopened, true)}, nil
}
