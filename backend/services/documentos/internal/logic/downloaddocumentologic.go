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

type DownloadDocumentoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDownloadDocumentoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DownloadDocumentoLogic {
	return &DownloadDocumentoLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *DownloadDocumentoLogic) DownloadDocumento(in *documentos.DownloadDocumentoRequest) (*documentos.DownloadDocumentoResponse, error) {
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
	// Paso 19D, cuál de los dos aplicó decide si además hace falta
	// verificar ownership real más abajo — mismo criterio ya usado en
	// GetDocumento (Paso 19B) y ListDocumentos (Paso 19C).
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

	// Metadata primero (reutiliza GetMetadata, ya existente — sin SQL
	// nuevo): alcanza para conocer expediente_id y resolver el ownership
	// SIN leer todavía el contenido binario del documento.
	meta, err := l.svcCtx.DocumentoRepository.GetMetadata(l.ctx, strings.TrimSpace(in.Id))
	if err != nil {
		if errors.Is(err, repository.ErrDocumentoNotFound) {
			return nil, status.Error(codes.NotFound, "documento no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar el documento")
	}

	// Ownership real (Paso 19D): solo cuando el acceso depende de
	// "documentos.view_own" (sin "documentos.view" global). SubidoPor NO
	// se usa como ownership -- representa únicamente quién cargó el
	// archivo, no el dueño del expediente (ver Paso 19, caso del PROVEIDO
	// subido por Dirección). Se delega en Expedientes.GetExpediente,
	// reenviando el mismo JWT — mismo patrón exacto que GetDocumento
	// (Paso 19B) y ListDocumentos (Paso 19C), sin cliente ni mecanismo
	// nuevo. Esta protección es ADICIONAL a la que ya existe en el
	// Gateway (composición con Expedientes.GetExpediente antes de llamar
	// a este RPC) — ninguna de las dos se elimina; ahora hay defensa en
	// profundidad: el Gateway protege el flujo HTTP, Documentos protege
	// su propio RPC aunque se le llame directamente.
	if !hasViewAll {
		outCtx := l.ctx
		if token, tokenOk := interceptor.TokenFromContext(l.ctx); tokenOk {
			outCtx = metadata.AppendToOutgoingContext(l.ctx, "authorization", "Bearer "+token)
		}
		if _, err := l.svcCtx.ExpedientesClient.GetExpediente(outCtx, &expedientesclient.GetExpedienteRequest{
			Id: meta.ExpedienteID,
		}); err != nil {
			return nil, err
		}
	}

	found, err := l.svcCtx.DocumentoRepository.GetWithContent(l.ctx, strings.TrimSpace(in.Id))
	if err != nil {
		if errors.Is(err, repository.ErrDocumentoNotFound) {
			return nil, status.Error(codes.NotFound, "documento no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo descargar el documento")
	}

	return &documentos.DownloadDocumentoResponse{
		Documento: toProto(&found.Documento),
		Contenido: found.Contenido,
	}, nil
}
