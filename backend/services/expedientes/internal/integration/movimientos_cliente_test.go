package integration

import (
	"context"
	"crypto/rand"
	"fmt"

	"expedientes/expedientes"
	"expedientes/internal/repository"

	"google.golang.org/grpc"
)

// clienteMovimientos envuelve el cliente gRPC para las pruebas anteriores a
// A + B: completa los campos nuevos obligatorios (idempotency_key y área o
// estado esperados) con el estado REAL del expediente leído de la base, de
// modo que cada caso conserva exactamente lo que verificaba. Un valor
// explícito del caso nunca se reemplaza (así se prueban lecturas viejas).
type clienteMovimientos struct {
	expedientes.ExpedientesClient
	repo *repository.ExpedienteRepository
}

func nuevaClaveTest() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (c clienteMovimientos) actual(id string) (area, estado string) {
	e, err := c.repo.FindByIDOrCodigo(context.Background(), id)
	if err != nil {
		return "SECRETARIA", "PENDIENTE" // inexistente/inactivo: el RPC devolverá NotFound
	}
	return e.AreaActual, e.Estado
}

func (c clienteMovimientos) DerivarExpediente(ctx context.Context, in *expedientes.DerivarExpedienteRequest, opts ...grpc.CallOption) (*expedientes.DerivarExpedienteResponse, error) {
	r := &expedientes.DerivarExpedienteRequest{Id: in.Id, AreaDestino: in.AreaDestino, Motivo: in.Motivo,
		Condicion: in.Condicion, AreaOrigenEsperada: in.AreaOrigenEsperada, IdempotencyKey: in.IdempotencyKey}
	if r.AreaOrigenEsperada == "" {
		r.AreaOrigenEsperada, _ = c.actual(r.Id)
	}
	if r.Motivo == "" {
		r.Motivo = "prueba de integración"
	}
	if r.IdempotencyKey == "" {
		r.IdempotencyKey = nuevaClaveTest()
	}
	return c.ExpedientesClient.DerivarExpediente(ctx, r, opts...)
}

func (c clienteMovimientos) RechazarExpediente(ctx context.Context, in *expedientes.RechazarExpedienteRequest, opts ...grpc.CallOption) (*expedientes.RechazarExpedienteResponse, error) {
	r := &expedientes.RechazarExpedienteRequest{Id: in.Id, Motivo: in.Motivo, AreaOrigenEsperada: in.AreaOrigenEsperada, IdempotencyKey: in.IdempotencyKey}
	if r.AreaOrigenEsperada == "" {
		r.AreaOrigenEsperada, _ = c.actual(r.Id)
	}
	if r.IdempotencyKey == "" {
		r.IdempotencyKey = nuevaClaveTest()
	}
	return c.ExpedientesClient.RechazarExpediente(ctx, r, opts...)
}

func (c clienteMovimientos) ChangeEstado(ctx context.Context, in *expedientes.ChangeEstadoRequest, opts ...grpc.CallOption) (*expedientes.ChangeEstadoResponse, error) {
	r := &expedientes.ChangeEstadoRequest{Id: in.Id, NuevoEstado: in.NuevoEstado, EstadoEsperado: in.EstadoEsperado, IdempotencyKey: in.IdempotencyKey, Motivo: in.Motivo}
	if r.EstadoEsperado == "" {
		_, r.EstadoEsperado = c.actual(r.Id)
	}
	if r.IdempotencyKey == "" {
		r.IdempotencyKey = nuevaClaveTest()
	}
	return c.ExpedientesClient.ChangeEstado(ctx, r, opts...)
}

func (c clienteMovimientos) ResolverExpediente(ctx context.Context, in *expedientes.ResolverExpedienteRequest, opts ...grpc.CallOption) (*expedientes.ResolverExpedienteResponse, error) {
	r := &expedientes.ResolverExpedienteRequest{Id: in.Id, IdempotencyKey: in.IdempotencyKey}
	if r.IdempotencyKey == "" {
		r.IdempotencyKey = nuevaClaveTest()
	}
	return c.ExpedientesClient.ResolverExpediente(ctx, r, opts...)
}

func (c clienteMovimientos) CorregirExpediente(ctx context.Context, in *expedientes.CorregirExpedienteRequest, opts ...grpc.CallOption) (*expedientes.CorregirExpedienteResponse, error) {
	r := &expedientes.CorregirExpedienteRequest{Id: in.Id, Descripcion: in.Descripcion, IdempotencyKey: in.IdempotencyKey}
	if r.IdempotencyKey == "" {
		r.IdempotencyKey = nuevaClaveTest()
	}
	return c.ExpedientesClient.CorregirExpediente(ctx, r, opts...)
}

// derivarLegado: las aserciones de autorización por área que antes usaban
// UpdateArea (deshabilitado en A + B) se ejercitan sobre su reemplazo,
// DerivarExpediente, con el área actual real como origen esperado.
func (c clienteMovimientos) derivarLegado(ctx context.Context, in *expedientes.UpdateAreaRequest) (*expedientes.DerivarExpedienteResponse, error) {
	return c.DerivarExpediente(ctx, &expedientes.DerivarExpedienteRequest{Id: in.Id, AreaDestino: in.Area})
}
