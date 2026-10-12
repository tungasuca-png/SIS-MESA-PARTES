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

// Las tres lecturas comparten el mismo patrón: identidad del JWT -> alcance
// (permiso view / view_own) -> lectura -> auditoría VIEW (fail-closed).

type GetExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetExpedienteLogic {
	return &GetExpedienteLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetExpedienteLogic) GetExpediente(in *psicologia.GetExpedienteRequest) (*psicologia.GetExpedienteResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	id := ""
	if in != nil {
		id = strings.TrimSpace(in.Id)
	}
	if !validation.IsValidUUID(id) {
		return nil, status.Error(codes.InvalidArgument, "el identificador no es válido")
	}

	s, ok, err := resolveScope(l.ctx, l.svcCtx, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, denied(l.ctx, l.svcCtx, userID, id, "", motivoPermisoInsuficiente)
	}

	exp, err := l.svcCtx.ExpedienteRepo.GetByID(l.ctx, id)
	if err != nil {
		return nil, repoError(l.ctx, err, "no se pudo consultar el expediente psicológico")
	}
	return finishRead(l.ctx, l.svcCtx, s, userID, exp)
}

type GetExpedienteByEstudianteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetExpedienteByEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetExpedienteByEstudianteLogic {
	return &GetExpedienteByEstudianteLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GetExpedienteByEstudiante nunca crea un expediente: si no existe, NotFound.
func (l *GetExpedienteByEstudianteLogic) GetExpedienteByEstudiante(in *psicologia.GetExpedienteByEstudianteRequest) (*psicologia.GetExpedienteByEstudianteResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}
	estudianteID := ""
	if in != nil {
		estudianteID = strings.TrimSpace(in.EstudianteId)
	}
	if !validation.IsValidUUID(estudianteID) {
		return nil, status.Error(codes.InvalidArgument, "el estudiante_id no es válido")
	}

	s, ok, err := resolveScope(l.ctx, l.svcCtx, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, denied(l.ctx, l.svcCtx, userID, "", estudianteID, motivoPermisoInsuficiente)
	}

	exp, err := l.svcCtx.ExpedienteRepo.GetByEstudiante(l.ctx, estudianteID)
	if err != nil {
		return nil, repoError(l.ctx, err, "no se pudo consultar el expediente psicológico")
	}
	resp, err := finishRead(l.ctx, l.svcCtx, s, userID, exp)
	if err != nil {
		return nil, err
	}
	return &psicologia.GetExpedienteByEstudianteResponse{Expediente: resp.Expediente}, nil
}

// finishRead aplica el alcance sobre el expediente ya cargado y audita VIEW.
func finishRead(ctx context.Context, sc *svc.ServiceContext, s scope, userID string, exp *repository.Expediente) (*psicologia.GetExpedienteResponse, error) {
	if !s.allows(exp, userID) {
		return nil, notFoundHidden(ctx, sc, userID, exp.ID, exp.EstudianteID)
	}
	if err := auditView(ctx, sc, userID, exp.ID, exp.EstudianteID, ""); err != nil {
		return nil, err
	}
	return &psicologia.GetExpedienteResponse{Expediente: toProto(exp, true)}, nil
}

type ListExpedientesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListExpedientesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListExpedientesLogic {
	return &ListExpedientesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// ListExpedientes filtra por alcance EN SQL: con view_own solo devuelve los
// expedientes donde el usuario es responsable. No incluye resumen_caso.
func (l *ListExpedientesLogic) ListExpedientes(in *psicologia.ListExpedientesRequest) (*psicologia.ListExpedientesResponse, error) {
	userID, err := authenticatedUser(l.ctx)
	if err != nil {
		return nil, err
	}

	filter := repository.ListFilter{Page: 1, PageSize: defaultPageSize}
	if in != nil {
		filter.Estado = strings.ToUpper(strings.TrimSpace(in.Estado))
		if in.Page > 0 {
			filter.Page = int(in.Page)
		}
		if in.PageSize > 0 && in.PageSize <= maxPageSize {
			filter.PageSize = int(in.PageSize)
		}
	}
	if filter.Estado != "" && !validation.IsEstado(filter.Estado) {
		return nil, status.Error(codes.InvalidArgument, "el estado no es válido")
	}

	s, ok, err := resolveScope(l.ctx, l.svcCtx, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, denied(l.ctx, l.svcCtx, userID, "", "", motivoPermisoInsuficiente)
	}
	filter.ResponsableID = s.responsableFilter(userID)

	items, total, err := l.svcCtx.ExpedienteRepo.List(l.ctx, filter)
	if err != nil {
		return nil, repoError(l.ctx, err, "no se pudo listar los expedientes psicológicos")
	}
	if err := auditView(l.ctx, l.svcCtx, userID, "", "", motivoListado); err != nil {
		return nil, err
	}

	out := make([]*psicologia.ExpedientePsicologico, 0, len(items))
	for _, item := range items {
		out = append(out, toProto(item, false))
	}
	return &psicologia.ListExpedientesResponse{Expedientes: out, Total: int32(total)}, nil
}
