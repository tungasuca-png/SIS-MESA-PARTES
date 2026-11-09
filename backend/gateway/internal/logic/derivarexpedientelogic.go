// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"strings"

	"expedientes/expedientesclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DerivarExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDerivarExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DerivarExpedienteLogic {
	return &DerivarExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DerivarExpediente (A + B): UNA sola llamada a Expedientes, que mueve el área
// y registra el movimiento (con su evento de outbox) en la misma transacción.
// Ya no hay una segunda escritura en Derivaciones desde aquí (causa de
// H-01/H-02): la proyección llega por el publicador del outbox.
// Idempotency-Key + area_origen_esperada: un reintento con la misma clave
// devuelve el MISMO movimiento (repetido = true), nunca otro.
func (l *DerivarExpedienteLogic) DerivarExpediente(req *types.DerivarExpedienteRequest) (resp *types.DerivarExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	// Etapa 4: Subdirección -> Docente en F4 exige el proveído ya cargado. Solo
	// se comprueba cuando aplica. Si el actor ya no puede leer el expediente
	// (p. ej. reintento de una derivación que YA se hizo), se delega en
	// Expedientes: devuelve el resultado repetido o deniega por área, así que
	// la precondición no puede saltarse.
	if strings.EqualFold(strings.TrimSpace(req.AreaOrigenEsperada), "SUBDIRECTOR") &&
		strings.EqualFold(strings.TrimSpace(req.AreaDestino), "DOCENTE") {
		before, err := l.svcCtx.ExpedientesClient.GetExpediente(ctx, &expedientesclient.GetExpedienteRequest{Id: req.Id})
		switch {
		case err == nil:
			tipo := before.Expediente.Tipo
			if (tipo == "PERMISO" || tipo == "JUSTIFICACION_FALTA" || tipo == "JUSTIFICACION_TARDANZA") &&
				before.Expediente.AreaActual == "SUBDIRECTOR" {
				if err := requireDocumentoProveido(ctx, l.svcCtx, before.Expediente.Id); err != nil {
					return nil, err
				}
			}
		case status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.NotFound:
			// sin acceso: lo resuelve Expedientes (repetido o denegado)
		default:
			return nil, errorMovimiento(err)
		}
	}

	moved, err := l.svcCtx.ExpedientesClient.DerivarExpediente(ctx, &expedientesclient.DerivarExpedienteRequest{
		Id:                 req.Id,
		AreaDestino:        req.AreaDestino,
		Motivo:             req.Motivo,
		Condicion:          req.Condicion,
		AreaOrigenEsperada: req.AreaOrigenEsperada,
		IdempotencyKey:     req.IdempotencyKey,
	})
	if err != nil {
		return nil, errorMovimiento(err)
	}
	return &types.DerivarExpedienteResponse{
		Expediente: toExpedienteDTO(moved.Expediente),
		Movimiento: toMovimientoDTO(moved.Movimiento),
		Repetido:   moved.Repetido,
		Derivacion: derivacionDeMovimiento(moved.Movimiento),
	}, nil
}
