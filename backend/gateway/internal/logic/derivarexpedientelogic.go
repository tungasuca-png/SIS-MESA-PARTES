// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"derivaciones/derivacionesclient"
	"expedientes/expedientesclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
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

// DerivarExpediente (Etapa 3) reemplaza el patrón anterior del frontend
// (crearDerivacion() + updateArea() por separado, sin garantía de que
// ambas tuvieran éxito — ver docs/etapa-1.1-cierre-seguridad.md punto 2).
// Orden elegido a propósito: primero se mueve el área (autoritativo, con
// la transición validada por Expedientes Service — ver estados.CanDerivar)
// y solo si eso tuvo éxito se registra la derivación como historial. Un
// área movida sin su log puntual es un problema menor que un log que
// "miente" sobre un movimiento que nunca ocurrió (el bug ya documentado en
// Etapa 1.1).
func (l *DerivarExpedienteLogic) DerivarExpediente(req *types.DerivarExpedienteRequest) (resp *types.DerivarExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	// Se necesita el área ANTES de moverla para registrar el origen real en
	// el historial (DerivarExpediente de Expedientes Service solo devuelve
	// el expediente YA actualizado). Esta misma llamada además reutiliza la
	// validación de propiedad de área ya existente (Etapa 1).
	before, err := l.svcCtx.ExpedientesClient.GetExpediente(ctx, &expedientesclient.GetExpedienteRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}
	origen := before.Expediente.AreaActual

	// Etapa 4: Subdirección -> Docente en F4 exige que el proveído ya esté
	// cargado ("generar/formalizar proveído" es el paso 1, "enviarlo al
	// docente" es el paso 3 de la regla institucional — no puede derivarse
	// sin que el paso 1 ya haya ocurrido). Se verifica ANTES de mover nada.
	esCierreF4ADocente := (before.Expediente.Tipo == "PERMISO" ||
		before.Expediente.Tipo == "JUSTIFICACION_FALTA" ||
		before.Expediente.Tipo == "JUSTIFICACION_TARDANZA") &&
		origen == "SUBDIRECTOR" && req.AreaDestino == "DOCENTE"
	if esCierreF4ADocente {
		if err := requireDocumentoProveido(ctx, l.svcCtx, before.Expediente.Id); err != nil {
			return nil, err
		}
	}

	moved, err := l.svcCtx.ExpedientesClient.DerivarExpediente(ctx, &expedientesclient.DerivarExpedienteRequest{
		Id:          req.Id,
		AreaDestino: req.AreaDestino,
	})
	if err != nil {
		return nil, err
	}

	created, err := l.svcCtx.DerivacionesClient.CrearDerivacion(ctx, &derivacionesclient.CrearDerivacionRequest{
		ExpedienteId: req.Id,
		Tipo:         "DERIVACION",
		Origen:       origen,
		Destino:      req.AreaDestino,
		Motivo:       req.Motivo,
		Condicion:    req.Condicion,
	})
	if err != nil {
		// El área YA se movió correctamente (moved.Expediente ya refleja el
		// nuevo area_actual) — no se revierte: revertir podría chocar con
		// otra operación legítima que haya ocurrido justo después, y
		// "deshacer" no es más seguro que quedarse sin el log puntual (ver
		// comentario de la función). Se avisa explícitamente del estado
		// parcial para que quede visible, no silencioso.
		return nil, status.Errorf(
			status.Code(err),
			"el expediente se derivó correctamente a %s, pero no se pudo registrar el historial: %v",
			req.AreaDestino, err,
		)
	}

	return &types.DerivarExpedienteResponse{
		Expediente: toExpedienteDTO(moved.Expediente),
		Derivacion: toDerivacionDTO(created.Derivacion),
	}, nil
}
