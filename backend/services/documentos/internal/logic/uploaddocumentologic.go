package logic

import (
	"context"
	"strings"

	"documentos/documentos"
	"documentos/internal/authorization"
	"documentos/internal/interceptor"
	"documentos/internal/svc"
	"documentos/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UploadDocumentoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUploadDocumentoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadDocumentoLogic {
	return &UploadDocumentoLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UploadDocumentoLogic) UploadDocumento(in *documentos.UploadDocumentoRequest) (*documentos.UploadDocumentoResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "la solicitud es obligatoria")
	}

	expedienteID := strings.TrimSpace(in.ExpedienteId)
	if !validation.IsValidUUID(expedienteID) {
		return nil, status.Error(codes.InvalidArgument, "el expediente_id no es válido")
	}

	nombre := strings.TrimSpace(in.Nombre)
	if nombre == "" {
		return nil, status.Error(codes.InvalidArgument, "el nombre del documento es obligatorio")
	}
	if len(nombre) > 255 {
		return nil, status.Error(codes.InvalidArgument, "el nombre no puede superar los 255 caracteres")
	}

	tipoDocumento := strings.ToUpper(strings.TrimSpace(in.TipoDocumento))
	if !validation.IsValidTipoDocumento(tipoDocumento) {
		return nil, status.Error(codes.InvalidArgument, "el tipo de documento no es válido")
	}
	if !authorization.CanUpload(role, tipoDocumento) {
		return nil, status.Error(codes.PermissionDenied, "el rol no puede subir este tipo de documento")
	}

	extension := strings.ToLower(strings.TrimSpace(in.Extension))
	if !validation.IsValidExtension(extension) {
		return nil, status.Error(codes.InvalidArgument, "la extensión del archivo no es válida")
	}

	if len(in.Contenido) == 0 {
		return nil, status.Error(codes.InvalidArgument, "el archivo está vacío")
	}
	if len(in.Contenido) > validation.MaxTamanoDocumento {
		return nil, status.Error(codes.InvalidArgument, "el archivo supera el tamaño máximo permitido")
	}

	created, err := l.svcCtx.DocumentoRepository.Upload(l.ctx, expedienteID, nombre, tipoDocumento, extension, userID, in.Contenido)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo guardar el documento")
	}

	return &documentos.UploadDocumentoResponse{Documento: toProto(created)}, nil
}
