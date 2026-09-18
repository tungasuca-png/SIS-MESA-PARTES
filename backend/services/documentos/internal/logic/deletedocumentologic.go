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

// estadoPendienteExpediente replica el valor real de estados.EstadoPendiente
// (Expedientes Service, internal/estados/estados.go) — ese paquete es
// "internal" del módulo expedientes, así que Documentos (un módulo Go
// distinto) no puede importarlo (el compilador lo rechaza: "use of
// internal package ... not allowed"). No existe ninguna constante
// equivalente expuesta por expedientesclient (el cliente generado solo
// expone los mensajes del proto, no los valores de estados.go). El campo
// Estado del proto Expediente es un simple string (no un enum Go
// compartido), así que comparar contra el valor real y documentado
// ("PENDIENTE", el único de los 4 estados reales que permite eliminar)
// es la única forma de aplicar la regla del Paso 19E sin duplicar la
// lógica de transición de estados (CanTransition, etc.) ni crear un RPC
// nuevo.
const estadoPendienteExpediente = "PENDIENTE"

type DeleteDocumentoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteDocumentoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteDocumentoLogic {
	return &DeleteDocumentoLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *DeleteDocumentoLogic) DeleteDocumento(in *documentos.DeleteDocumentoRequest) (*documentos.DeleteDocumentoResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	// Permiso real (Paso 14B) + regla de negocio existente (CanDelete),
	// ambas deben cumplirse — mismo alcance que UpdateExpediente/
	// DeleteExpediente en Expedientes (Paso 13B).
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "documentos.delete"); err != nil {
		return nil, err
	}
	if !authorization.CanDelete(role) {
		return nil, status.Error(codes.PermissionDenied, "el rol no puede eliminar documentos")
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	// Área y estado (Paso 19E): el permiso ya confirmó que el rol puede
	// eliminar EN GENERAL (solo lo tienen los 6 roles internos — ningún
	// SOLICITANTE llega hasta aquí); falta confirmar que ESTE documento,
	// vía el expediente al que pertenece, le corresponde a este actor y
	// que el expediente sigue en un estado que permite eliminarlo.
	//
	// Metadata primero (reutiliza GetMetadata, ya existente — sin SQL
	// nuevo) para conocer expediente_id SIN leer todavía el contenido
	// binario (que Delete tampoco necesita).
	documentoID := strings.TrimSpace(in.Id)
	meta, err := l.svcCtx.DocumentoRepository.GetMetadata(l.ctx, documentoID)
	if err != nil {
		if errors.Is(err, repository.ErrDocumentoNotFound) {
			return nil, status.Error(codes.NotFound, "documento no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar el documento")
	}

	// Área: se delega en Expedientes.GetExpediente, reenviando el mismo
	// JWT — mismo patrón exacto ya usado para ownership en GetDocumento/
	// DownloadDocumento/ListDocumentos (Pasos 19B/19C/19D). El llamador
	// aquí SIEMPRE es un rol interno (es la única forma de tener
	// "documentos.delete"), así que GetExpediente aplica exactamente la
	// misma regla pedida: CanViewAll (ADMIN, y también SECRETARIA por ser
	// la mesa de partes — mismo criterio ya vigente en TODAS las demás
	// operaciones de Expedientes sobre expedientes, ver Pasos 13B/18B/18C)
	// ve/edita cualquier área; el resto de internos solo si
	// AreaActual == AreaDelRol(role) — sin importar esa función acá, sin
	// duplicar el mapeo rol->área (imposible de todos modos: AreaDelRol
	// vive en expedientes/internal/authorization, un paquete "internal"
	// del módulo expedientes, que Documentos —un módulo Go distinto— no
	// puede importar). Si el área no corresponde, GetExpediente devuelve
	// PermissionDenied por sí solo; si el expediente no existe, NotFound;
	// si Expedientes no responde, el error de comunicación se propaga
	// (fail-closed).
	outCtx := l.ctx
	if token, tokenOk := interceptor.TokenFromContext(l.ctx); tokenOk {
		outCtx = metadata.AppendToOutgoingContext(l.ctx, "authorization", "Bearer "+token)
	}
	expedienteResp, err := l.svcCtx.ExpedientesClient.GetExpediente(outCtx, &expedientesclient.GetExpedienteRequest{
		Id: meta.ExpedienteID,
	})
	if err != nil {
		return nil, err
	}

	// Estado (Paso 19E): solo se puede eliminar un documento cuyo
	// expediente sigue PENDIENTE (mismo criterio institucional ya
	// aplicado a DeleteExpediente, Paso 18C). No se inventa una regla
	// especial para PROVEIDO: si algún día un PROVEIDO pudiera existir en
	// PENDIENTE, esta misma regla ya lo permitiría eliminar, sin
	// necesidad de un caso aparte.
	if expedienteResp.Expediente.Estado != estadoPendienteExpediente {
		return nil, status.Error(codes.PermissionDenied, "solo se puede eliminar un documento de un expediente pendiente")
	}

	if err := l.svcCtx.DocumentoRepository.Delete(l.ctx, documentoID); err != nil {
		if errors.Is(err, repository.ErrDocumentoNotFound) {
			return nil, status.Error(codes.NotFound, "documento no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo eliminar el documento")
	}

	return &documentos.DeleteDocumentoResponse{Success: true}, nil
}
