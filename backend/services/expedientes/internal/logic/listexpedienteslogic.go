package logic

import (
	"context"
	"strings"

	"expedientes/expedientes"
	"expedientes/internal/authorization"
	"expedientes/internal/estados"
	"expedientes/internal/interceptor"
	"expedientes/internal/repository"
	"expedientes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type ListExpedientesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListExpedientesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListExpedientesLogic {
	return &ListExpedientesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListExpedientesLogic) ListExpedientes(in *expedientes.ListExpedientesRequest) (*expedientes.ListExpedientesResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	// Permiso real (Paso 13B): decide SI puede pedir este tipo de consulta.
	// Las reglas de abajo (CanViewAll/AreaDelRol/SolicitanteID) siguen
	// decidiendo, sin cambios, CUÁLES expedientes recibe.
	listPermission := "expedientes.view"
	if !authorization.IsInternal(role) {
		listPermission = "expedientes.view_own"
	}
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, listPermission); err != nil {
		return nil, err
	}

	filter := repository.ListFilter{
		Estado:    strings.ToUpper(strings.TrimSpace(in.GetEstado())),
		Prioridad: strings.ToUpper(strings.TrimSpace(in.GetPrioridad())),
		Tipo:      strings.ToUpper(strings.TrimSpace(in.GetTipo())),
		Page:      int(in.GetPage()),
		PageSize:  int(in.GetPageSize()),
	}
	if filter.Estado != "" && !estados.IsValidEstado(filter.Estado) {
		return nil, status.Error(codes.InvalidArgument, "el estado indicado no es válido")
	}
	if filter.Prioridad != "" && !estados.IsValidPrioridad(filter.Prioridad) {
		return nil, status.Error(codes.InvalidArgument, "la prioridad indicada no es válida")
	}
	if filter.Tipo != "" && !estados.IsValidTipo(filter.Tipo) {
		return nil, status.Error(codes.InvalidArgument, "el tipo indicado no es válido")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > maxPageSize {
		filter.PageSize = defaultPageSize
	}

	// Secretaría (mesa de partes) y Admin ven todo. El resto del personal
	// interno solo ve lo que ya se le derivó a su propia área; un
	// SOLICITANTE (externo) solo ve lo que presentó él mismo.
	if !authorization.CanViewAll(role) {
		if authorization.IsInternal(role) {
			filter.AreaActual = authorization.AreaDelRol(role)
		} else {
			filter.SolicitanteID = userID
		}
	}

	items, total, err := l.svcCtx.ExpedienteRepository.List(l.ctx, filter)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo listar los expedientes")
	}

	protoItems := make([]*expedientes.Expediente, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, toProto(item))
	}

	return &expedientes.ListExpedientesResponse{
		Expedientes: protoItems,
		Total:       int32(total),
	}, nil
}
