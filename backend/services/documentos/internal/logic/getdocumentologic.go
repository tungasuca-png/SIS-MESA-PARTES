package logic

import (
	"context"
	"errors"
	"strings"

	"documentos/documentos"
	"documentos/internal/authorization"
	"documentos/internal/interceptor"
	"documentos/internal/repository"
	"documentos/internal/svc"

	"expedientes/expedientesclient"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type GetDocumentoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDocumentoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDocumentoLogic {
	return &GetDocumentoLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetDocumentoLogic) GetDocumento(in *documentos.GetDocumentoRequest) (*documentos.GetDocumentoResponse, error) {
	userID, _, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	// Permiso real (Paso 14B): "documentos.view" (acceso global, SIN
	// cambios en este paso) o "documentos.view_own". Se consultan por
	// separado (en vez de RequireAnyPermission) porque, a partir del
	// Paso 19B, cuál de los dos aplicó decide si además hace falta
	// verificar ownership real más abajo.
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
			return nil, status.Error(codes.PermissionDenied, "el usuario no tiene ninguno de los permisos requeridos")
		}
	}

	found, err := l.svcCtx.DocumentoRepository.GetMetadata(l.ctx, strings.TrimSpace(in.Id))
	if err != nil {
		if errors.Is(err, repository.ErrDocumentoNotFound) {
			return nil, status.Error(codes.NotFound, "documento no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar el documento")
	}

	// Ownership real (Paso 19B): solo cuando el acceso depende de
	// "documentos.view_own" (sin "documentos.view" global). SubidoPor NO
	// se usa como ownership -- representa únicamente quién cargó el
	// archivo, no el dueño del expediente (ver Paso 19, hallazgo del
	// problema institucional del PROVEIDO). Se delega la verificación a
	// Expedientes.GetExpediente, reenviando el MISMO JWT que autenticó
	// esta llamada: ese RPC ya aplica su propia regla de ownership
	// (SolicitanteID == usuario autenticado, para un rol no interno) —
	// si el expediente no le pertenece, GetExpediente por sí solo
	// devuelve PermissionDenied, sin duplicar esa comparación acá. Fail-
	// closed: cualquier error (Expedientes no disponible incluido) se
	// propaga tal cual, nunca se traduce en acceso permitido.
	if !hasViewAll {
		outCtx := l.ctx
		if token, tokenOk := interceptor.TokenFromContext(l.ctx); tokenOk {
			outCtx = metadata.AppendToOutgoingContext(l.ctx, "authorization", "Bearer "+token)
		}
		if _, err := l.svcCtx.ExpedientesClient.GetExpediente(outCtx, &expedientesclient.GetExpedienteRequest{
			Id: found.ExpedienteID,
		}); err != nil {
			return nil, err
		}
	}

	return &documentos.GetDocumentoResponse{Documento: toProto(found)}, nil
}
