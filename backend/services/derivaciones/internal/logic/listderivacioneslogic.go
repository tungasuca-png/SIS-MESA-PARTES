package logic

import (
	"context"
	"strings"

	"derivaciones/derivaciones"
	"derivaciones/internal/authorization"
	"derivaciones/internal/interceptor"
	"derivaciones/internal/repository"
	"derivaciones/internal/svc"
	"derivaciones/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type ListDerivacionesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDerivacionesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDerivacionesLogic {
	return &ListDerivacionesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListDerivacionesLogic) ListDerivaciones(in *derivaciones.ListDerivacionesRequest) (*derivaciones.ListDerivacionesResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}

	filter := repository.ListFilter{}
	if in != nil {
		filter.ExpedienteID = strings.TrimSpace(in.ExpedienteId)
		filter.Tipo = strings.ToUpper(strings.TrimSpace(in.Tipo))
		filter.Page = int(in.Page)
		filter.PageSize = int(in.PageSize)
	}
	if filter.ExpedienteID != "" && !validation.IsValidUUID(filter.ExpedienteID) {
		return nil, status.Error(codes.InvalidArgument, "el expediente_id no es válido")
	}
	if filter.Tipo != "" && !validation.IsValidTipo(filter.Tipo) {
		return nil, status.Error(codes.InvalidArgument, "el tipo de derivación no es válido")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > maxPageSize {
		filter.PageSize = defaultPageSize
	}

	// Permiso (Auth) y alcance (Expedientes) son responsabilidades separadas:
	// "derivaciones.view" -> alcance general; "derivaciones.view_own" -> solo
	// expedientes propios; ninguno -> PermissionDenied (sin importar el rol).
	vis, err := resolverVisibilidad(l.ctx, l.svcCtx, userID)
	if err != nil {
		return nil, err
	}
	if filter.ExpedienteID == "" && !vis.general {
		return nil, status.Error(codes.InvalidArgument, "el expediente_id es obligatorio")
	}
	if filter.ExpedienteID != "" {
		if err := vis.verificarExpediente(l.ctx, l.svcCtx, userID, filter.ExpedienteID); err != nil {
			return nil, err
		}
	} else if !authorization.AlcanceGlobalExpedientes(role) {
		// Listado general con "derivaciones.view": solo las derivaciones de
		// los expedientes que el usuario puede ver. Antes devolvía las de
		// toda la institución (con motivo y condición) a cualquier rol con
		// ese permiso, aunque Expedientes lo acota a su área.
		ids, err := expedientesVisibles(l.ctx, l.svcCtx)
		if err != nil {
			return nil, err
		}
		filter.ExpedienteIDs = ids
	}

	items, total, err := l.svcCtx.DerivacionRepository.List(l.ctx, filter)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo listar las derivaciones")
	}

	protoItems := make([]*derivaciones.Derivacion, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, vis.proyectar(toProto(item)))
	}

	return &derivaciones.ListDerivacionesResponse{Derivaciones: protoItems, Total: int32(total)}, nil
}
