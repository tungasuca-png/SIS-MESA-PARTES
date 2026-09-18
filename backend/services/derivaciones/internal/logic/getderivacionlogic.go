package logic

import (
	"context"
	"errors"
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

type GetDerivacionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDerivacionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDerivacionLogic {
	return &GetDerivacionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDerivacionLogic) GetDerivacion(in *derivaciones.GetDerivacionRequest) (*derivaciones.GetDerivacionResponse, error) {
	userID, _, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	// Permiso real (Paso 15B): antes no existía ninguna autorización más
	// allá de estar autenticado (hallazgo 🔴 del Paso 15 — cualquier
	// autenticado podía leer cualquier derivación por UUID). No se
	// inventa ownership nuevo: "derivaciones.view" es el único permiso de
	// lectura que existe en el catálogo actual.
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "derivaciones.view"); err != nil {
		return nil, err
	}
	if in == nil || strings.TrimSpace(in.Id) == "" || !validation.IsValidUUID(strings.TrimSpace(in.Id)) {
		return nil, status.Error(codes.InvalidArgument, "el identificador no es válido")
	}

	found, err := l.svcCtx.DerivacionRepository.Get(l.ctx, strings.TrimSpace(in.Id))
	if err != nil {
		if errors.Is(err, repository.ErrDerivacionNotFound) {
			return nil, status.Error(codes.NotFound, "derivación no encontrada")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar la derivación")
	}

	// Acceso al expediente real (Paso 20C): cierra el bypass detectado en
	// el Paso 20 — "derivaciones.view" ya no basta por sí solo; además
	// hay que demostrar acceso legítimo al expediente_id REAL de esta
	// derivación (found.ExpedienteID, nunca algo enviado por el cliente).
	// Se delega en Expedientes.GetExpediente, reenviando el mismo JWT —
	// mismo patrón exacto ya usado en ListDerivaciones (Paso 20A) y
	// CrearDerivacion (Paso 20B): esa llamada ya aplica su propia regla de
	// ownership (SolicitanteID, para SOLICITANTE) y de área
	// (CanViewAll/AreaDelRol, para un rol interno) — no se duplica esa
	// lógica acá, ni se importa. Fail-closed: cualquier error (expediente
	// inexistente -> NotFound, sin acceso -> PermissionDenied, Expedientes
	// no disponible -> error de comunicación) se propaga tal cual, ANTES
	// de devolver ningún dato de la derivación.
	outCtx := l.ctx
	if token, tokenOk := interceptor.TokenFromContext(l.ctx); tokenOk {
		outCtx = metadata.AppendToOutgoingContext(l.ctx, "authorization", "Bearer "+token)
	}
	if _, err := l.svcCtx.ExpedientesClient.GetExpediente(outCtx, &expedientesclient.GetExpedienteRequest{
		Id: found.ExpedienteID,
	}); err != nil {
		return nil, err
	}

	return &derivaciones.GetDerivacionResponse{Derivacion: toProto(found)}, nil
}
