package logic

import (
	"context"
	"errors"
	"strings"

	"estudiantes/estudiantes"
	"estudiantes/internal/authorization"
	"estudiantes/internal/interceptor"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetEstudianteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetEstudianteLogic {
	return &GetEstudianteLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GetEstudiante (FASE 5B): autenticación + permiso real
// ("estudiantes.view", vía Auth.HasPermission). Fail-closed: cualquier error
// de Auth (caído, timeout, respuesta inválida) se propaga tal cual — nunca
// se traduce en acceso permitido.
func (l *GetEstudianteLogic) GetEstudiante(in *estudiantes.GetEstudianteRequest) (*estudiantes.GetEstudianteResponse, error) {
	userID, _, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "estudiantes.view"); err != nil {
		return nil, err
	}
	id := ""
	if in != nil {
		id = strings.TrimSpace(in.Id)
	}
	if !validation.IsValidUUID(id) {
		return nil, status.Error(codes.InvalidArgument, "el identificador no es válido")
	}

	found, err := l.svcCtx.EstudianteRepository.GetByID(l.ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrEstudianteNotFound) {
			return nil, status.Error(codes.NotFound, "estudiante no encontrado")
		}
		l.Errorf("consultar estudiante %s: %s", id, repository.SafeErrorSummary(err))
		return nil, status.Error(codes.Internal, "no se pudo consultar el estudiante")
	}

	return &estudiantes.GetEstudianteResponse{Estudiante: toProto(found)}, nil
}
