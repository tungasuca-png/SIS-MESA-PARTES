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

	"expedientes/expedientesclient"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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

	// Permiso real (Paso 15B), en el MISMO lugar donde ya decidía
	// CanViewAll(role) — sin cambios en esta rama (sección 2.A del Paso
	// 20A: "no cambiar innecesariamente"). El catálogo actual solo tiene
	// "derivaciones.view" (no existe un equivalente a "view_own"), y un
	// SOLICITANTE nunca lo tiene.
	hasViewAll, err := authorization.HasPermission(l.ctx, l.svcCtx.AuthClient, userID, "derivaciones.view")
	if err != nil {
		return nil, err
	}
	if filter.ExpedienteID == "" && !hasViewAll {
		return nil, status.Error(codes.InvalidArgument, "el expediente_id es obligatorio")
	}

	// Ownership/área real (Paso 20A): cierra la vulnerabilidad CRÍTICA del
	// Paso 20 — antes, con expediente_id presente, no se exigía NINGÚN
	// permiso ni verificación de pertenencia; cualquier autenticado (
	// incluso sin ningún rol) podía listar el historial de cualquier
	// expediente. Ahora:
	//   - Un rol INTERNO (IsInternal) además necesita "derivaciones.view"
	//     (Caso 6 del Paso 20A) — sin esto, ni siquiera se consulta el
	//     expediente.
	//   - CUALQUIER caller (interno o no) debe demostrar acceso real al
	//     expediente, delegado en Expedientes.GetExpediente reenviando el
	//     mismo JWT: ese RPC ya aplica su propia regla de ownership
	//     (SolicitanteID == usuario, para un rol no interno) y de área
	//     (CanViewAll/AreaDelRol, para un rol interno) — no se duplica esa
	//     lógica acá, ni se importa (AreaDelRol vive en
	//     expedientes/internal/authorization, inalcanzable desde este
	//     módulo — mismo hallazgo ya documentado en Documentos, Paso 19E).
	//     Si el expediente no existe, no pertenece al actor, o Expedientes
	//     no responde, el error se propaga tal cual (fail-closed) — nunca
	//     se traduce en una lista vacía ni en acceso permitido.
	if filter.ExpedienteID != "" {
		if authorization.IsInternal(role) && !hasViewAll {
			return nil, status.Error(codes.PermissionDenied, "el usuario no tiene el permiso requerido")
		}
		outCtx := l.ctx
		if token, tokenOk := interceptor.TokenFromContext(l.ctx); tokenOk {
			outCtx = metadata.AppendToOutgoingContext(l.ctx, "authorization", "Bearer "+token)
		}
		if _, err := l.svcCtx.ExpedientesClient.GetExpediente(outCtx, &expedientesclient.GetExpedienteRequest{
			Id: filter.ExpedienteID,
		}); err != nil {
			return nil, err
		}
	}

	items, total, err := l.svcCtx.DerivacionRepository.List(l.ctx, filter)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo listar las derivaciones")
	}

	protoItems := make([]*derivaciones.Derivacion, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, toProto(item))
	}

	return &derivaciones.ListDerivacionesResponse{Derivaciones: protoItems, Total: int32(total)}, nil
}
