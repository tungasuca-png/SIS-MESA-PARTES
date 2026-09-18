package logic

import (
	"context"
	"strings"

	"documentos/documentos"
	"documentos/internal/authorization"
	"documentos/internal/interceptor"
	"documentos/internal/repository"
	"documentos/internal/svc"
	"documentos/internal/validation"

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

type ListDocumentosLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDocumentosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDocumentosLogic {
	return &ListDocumentosLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListDocumentosLogic) ListDocumentos(in *documentos.ListDocumentosRequest) (*documentos.ListDocumentosResponse, error) {
	userID, _, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}

	filter := repository.ListFilter{}
	if in != nil {
		filter.ExpedienteID = strings.TrimSpace(in.ExpedienteId)
		filter.TipoDocumento = strings.ToUpper(strings.TrimSpace(in.TipoDocumento))
		filter.Page = int(in.Page)
		filter.PageSize = int(in.PageSize)
	}
	if filter.ExpedienteID != "" && !validation.IsValidUUID(filter.ExpedienteID) {
		return nil, status.Error(codes.InvalidArgument, "el expediente_id no es válido")
	}
	if filter.TipoDocumento != "" && !validation.IsValidTipoDocumento(filter.TipoDocumento) {
		return nil, status.Error(codes.InvalidArgument, "el tipo de documento no es válido")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > maxPageSize {
		filter.PageSize = defaultPageSize
	}

	// Permiso real (Paso 14B): decide si puede listar en general, y con
	// qué alcance. hasViewAll es, a partir del Paso 19C, la ÚNICA fuente
	// para decidir alcance global en este RPC (antes la rama sin
	// expediente_id usaba authorization.CanViewAll(role), una fuente
	// distinta que hoy coincidía por casualidad con el catálogo real).
	hasViewAll, err := authorization.HasPermission(l.ctx, l.svcCtx.AuthClient, userID, "documentos.view")
	if err != nil {
		return nil, err
	}
	if !hasViewAll {
		hasViewOwn, err := authorization.HasPermission(l.ctx, l.svcCtx.AuthClient, userID, "documentos.view_own")
		if err != nil {
			return nil, err
		}
		if !hasViewOwn {
			return nil, status.Error(codes.PermissionDenied, "el usuario no tiene permiso para listar documentos")
		}
	}

	// Ownership real (Paso 19C): SubidoPor ya NO decide qué puede ver quien
	// solo tiene "documentos.view_own" — representa únicamente quién cargó
	// el archivo, no el dueño del expediente (ver Paso 19, problema del
	// PROVEIDO subido por Dirección). El ownership se resuelve siempre
	// contra Expedientes Service, reenviando el mismo JWT.
	if !hasViewAll {
		outCtx := l.outgoingContextWithToken()
		if filter.ExpedienteID != "" {
			// Un expediente puntual: basta confirmar que ESE expediente es
			// del usuario (GetExpediente ya aplica su propia regla de
			// ownership para un rol no interno) — si no lo es, deniega por
			// sí solo, sin duplicar la comparación acá. Sin filtro
			// adicional: una vez confirmado el ownership, se listan TODOS
			// los documentos de ese expediente, sin importar quién los
			// subió.
			if _, err := l.svcCtx.ExpedientesClient.GetExpediente(outCtx, &expedientesclient.GetExpedienteRequest{
				Id: filter.ExpedienteID,
			}); err != nil {
				return nil, err
			}
		} else {
			// Sin expediente puntual: se necesita el conjunto de
			// expedientes propios del usuario. Expedientes.ListExpedientes,
			// llamado con el MISMO JWT, ya se acota por sí solo a
			// SolicitanteID == usuario autenticado para un rol no interno
			// (ver ListExpedientesLogic) — no hace falta ningún RPC nuevo
			// ni ningún JOIN entre bases de datos distintas.
			expedienteIDs, err := l.solicitanteExpedienteIDs(outCtx)
			if err != nil {
				return nil, err
			}
			filter.ExpedienteIDs = expedienteIDs
		}
	}

	items, total, err := l.svcCtx.DocumentoRepository.List(l.ctx, filter)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo listar los documentos")
	}

	protoItems := make([]*documentos.Documento, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, toProto(item))
	}

	return &documentos.ListDocumentosResponse{Documentos: protoItems, Total: int32(total)}, nil
}

// outgoingContextWithToken reenvía el mismo JWT que autenticó esta llamada
// a Documentos hacia Expedientes Service — mismo mecanismo ya usado por
// authorization.HasPermission hacia Auth Service, y por GetDocumentoLogic
// (Paso 19B) hacia Expedientes.
func (l *ListDocumentosLogic) outgoingContextWithToken() context.Context {
	if token, ok := interceptor.TokenFromContext(l.ctx); ok {
		return metadata.AppendToOutgoingContext(l.ctx, "authorization", "Bearer "+token)
	}
	return l.ctx
}

// expedientesListPageSize pagina la llamada a Expedientes.ListExpedientes
// (máximo permitido por ese RPC, ver ListExpedientesLogic.maxPageSize) —
// evita depender de que un solicitante nunca tenga más de una página de
// expedientes propios, sin necesidad de ningún RPC nuevo.
const expedientesListPageSize = 100

// solicitanteExpedienteIDs obtiene TODOS los expedientes propios del
// usuario autenticado (paginando hasta agotar el total reportado), usando
// el RPC ya existente Expedientes.ListExpedientes — el número de llamadas
// depende de cuántos expedientes tiene ESE usuario, nunca de cuántos
// documentos existan (evita el patrón N+1 por documento).
func (l *ListDocumentosLogic) solicitanteExpedienteIDs(ctx context.Context) ([]string, error) {
	ids := make([]string, 0)
	for page := int32(1); ; page++ {
		resp, err := l.svcCtx.ExpedientesClient.ListExpedientes(ctx, &expedientesclient.ListExpedientesRequest{
			Page:     page,
			PageSize: expedientesListPageSize,
		})
		if err != nil {
			return nil, err
		}
		for _, exp := range resp.Expedientes {
			ids = append(ids, exp.Id)
		}
		if len(resp.Expedientes) == 0 || int32(len(ids)) >= resp.Total {
			break
		}
	}
	return ids, nil
}
