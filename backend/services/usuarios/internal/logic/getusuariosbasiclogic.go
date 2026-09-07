package logic

import (
	"context"
	"strings"

	"usuarios/internal/authorization"
	"usuarios/internal/interceptor"
	"usuarios/internal/svc"
	"usuarios/internal/validation"
	"usuarios/usuarios"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// maxBatchIDs limita la resolución por lote para evitar consultas masivas.
const maxBatchIDs = 100

type GetUsuariosBasicLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUsuariosBasicLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUsuariosBasicLogic {
	return &GetUsuariosBasicLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUsuariosBasicLogic) GetUsuariosBasic(in *usuarios.GetUsuariosBasicRequest) (*usuarios.GetUsuariosBasicResponse, error) {
	requesterID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if in == nil || len(in.Ids) == 0 {
		return &usuarios.GetUsuariosBasicResponse{Usuarios: []*usuarios.UsuarioBasic{}}, nil
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

	found, err := l.svcCtx.UsuarioRepository.FindManyByIDs(l.ctx, ids)
	if err != nil {
		l.Errorf("resolver usuarios por lote: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo consultar los usuarios")
	}

	// Los identificadores no encontrados simplemente no vienen en la
	// respuesta: el llamador decide cómo mostrar ese caso.
	items := make([]*usuarios.UsuarioBasic, 0, len(found))
	for _, item := range found {
		includeDNI := authorization.CanViewFullProfile(role, requesterID, item.ID)
		items = append(items, toProtoBasic(item, includeDNI))
	}

	return &usuarios.GetUsuariosBasicResponse{Usuarios: items}, nil
}
