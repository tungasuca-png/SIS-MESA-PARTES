package logic

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"expedientes/expedientes"
	"expedientes/internal/repository"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var patronUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// rolesConMovimiento: roles que pueden figurar como actor de un movimiento
// (chk_mov_actor_rol). Cualquier otro rol no puede mover expedientes.
var rolesConMovimiento = map[string]bool{
	"ADMIN": true, "DIRECTOR": true, "SUBDIRECTOR": true, "SECRETARIA": true,
	"DOCENTE": true, "AUXILIAR": true, "SOLICITANTE": true,
}

// validarClave exige una idempotency_key con formato UUID.
func validarClave(clave string) (string, error) {
	clave = strings.ToLower(strings.TrimSpace(clave))
	if !patronUUID.MatchString(clave) {
		return "", status.Error(codes.InvalidArgument, "la clave de idempotencia (UUID) es obligatoria")
	}
	return clave, nil
}

func validarRolActor(role string) error {
	if !rolesConMovimiento[role] {
		return status.Error(codes.PermissionDenied, "el rol no puede mover expedientes")
	}
	return nil
}

// contenido: representación canónica de la solicitud para la huella de
// idempotencia (separador de unidad, que no aparece en los datos).
func contenido(campos ...string) string {
	return strings.Join(campos, "\x1f")
}

func movimientoToProto(m *repository.Movimiento) *expedientes.Movimiento {
	if m == nil {
		return nil
	}
	return &expedientes.Movimiento{
		Id:             m.ID,
		ExpedienteId:   m.ExpedienteID,
		Secuencia:      int32(m.Secuencia),
		Tipo:           m.Tipo,
		ActorId:        m.ActorID,
		ActorRol:       m.ActorRol,
		AreaOrigen:     m.AreaOrigen,
		AreaDestino:    m.AreaDestino,
		EstadoAnterior: m.EstadoAnterior,
		EstadoNuevo:    m.EstadoNuevo,
		Motivo:         m.Motivo,
		Condicion:      m.Condicion,
		OcurridoEn:     m.OcurridoEn.UTC().Format(time.RFC3339Nano),
	}
}

// errorMovimiento traduce los errores de RegistrarMovimiento a códigos gRPC.
// Los status de las reglas de negocio (Decidir) pasan sin cambios.
func errorMovimiento(ctx context.Context, operacion string, err error) error {
	if _, ok := status.FromError(err); ok {
		return err
	}
	switch {
	case errors.Is(err, repository.ErrExpedienteNotFound):
		return status.Error(codes.NotFound, "expediente no encontrado")
	case errors.Is(err, repository.ErrExpedienteOcupado):
		return status.Error(codes.Aborted, "el expediente está siendo modificado por otra operación; intente nuevamente")
	case errors.Is(err, repository.ErrClaveReutilizada):
		return status.Error(codes.AlreadyExists, "la clave de idempotencia ya se usó con otra solicitud")
	case errors.Is(err, repository.ErrResultadoIncierto), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		logx.WithContext(ctx).Errorf("%s: resultado incierto: %v", operacion, err)
		return status.Error(codes.DeadlineExceeded, "no se pudo confirmar el resultado; reintente con la misma clave")
	}
	logx.WithContext(ctx).Errorf("%s: %v", operacion, err)
	return status.Error(codes.Internal, "no se pudo registrar el movimiento del expediente")
}
