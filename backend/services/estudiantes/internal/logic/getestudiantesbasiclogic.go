package logic

import (
	"context"
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

// maxBatchIDs limita la resolución por lote (mismo límite y mismo criterio
// que Usuarios.GetUsuariosBasic).
const maxBatchIDs = 100

type GetEstudiantesBasicLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetEstudiantesBasicLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetEstudiantesBasicLogic {
	return &GetEstudiantesBasicLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GetEstudiantesBasic (FASE 5B): autenticación + "estudiantes.view" (mismo
// criterio que GetEstudiante, ver su comentario). A diferencia de
// Usuarios.GetUsuariosBasic (deliberadamente sin permiso, para resolver
// nombres cruzados entre módulos), aquí el permiso SÍ se exige: son datos de
// estudiantes, no de cuentas del sistema. IDs no encontrados simplemente no
// vienen en la respuesta: el llamador decide cómo mostrar ese caso.
func (l *GetEstudiantesBasicLogic) GetEstudiantesBasic(in *estudiantes.GetEstudiantesBasicRequest) (*estudiantes.GetEstudiantesBasicResponse, error) {
	userID, _, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "estudiantes.view"); err != nil {
		return nil, err
	}
	if in == nil || len(in.Ids) == 0 {
		return &estudiantes.GetEstudiantesBasicResponse{Estudiantes: []*estudiantes.EstudianteBasico{}}, nil
	}
	if len(in.Ids) > maxBatchIDs {
		return nil, status.Error(codes.InvalidArgument, "demasiados identificadores en una sola consulta")
	}

	ids := make([]string, 0, len(in.Ids))
	seen := make(map[string]struct{}, len(in.Ids))
	for _, raw := range in.Ids {
		id := strings.TrimSpace(raw)
		if !validation.IsValidUUID(id) {
			return nil, status.Error(codes.InvalidArgument, "uno de los identificadores no es un UUID válido")
		}
		if _, duplicated := seen[id]; duplicated {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	found, err := l.svcCtx.EstudianteRepository.GetBasicByIDs(l.ctx, ids)
	if err != nil {
		l.Errorf("resolver estudiantes por lote: %s", repository.SafeErrorSummary(err))
		return nil, status.Error(codes.Internal, "no se pudo consultar los estudiantes")
	}

	items := make([]*estudiantes.EstudianteBasico, 0, len(found))
	for _, item := range found {
		items = append(items, toProtoBasic(item))
	}
	return &estudiantes.GetEstudiantesBasicResponse{Estudiantes: items}, nil
}
