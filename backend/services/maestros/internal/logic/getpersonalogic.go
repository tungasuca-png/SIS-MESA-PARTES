package logic

import (
	"context"
	"errors"
	"strings"

	"maestros/internal/svc"
	"maestros/maestros"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetPersonaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPersonaLogic(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
) *GetPersonaLogic {
	return &GetPersonaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetPersonaLogic) GetPersona(
	in *maestros.GetPersonaRequest,
) (*maestros.GetPersonaResponse, error) {

	// Validar que el ID haya sido enviado.
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"el id de la persona es obligatorio",
		)
	}

	personaID := strings.TrimSpace(in.Id)

	// Validar que el ID tenga formato UUID correcto.
	// Esto evita enviar valores inválidos a PostgreSQL.
	if _, err := uuid.Parse(personaID); err != nil {
		return nil, status.Error(
			codes.InvalidArgument,
			"el id de la persona debe ser un UUID válido",
		)
	}

	// Consultar la persona en maestros_db.
	p, err := l.svcCtx.PersonaRepository.FindByID(
		l.ctx,
		personaID,
	)

	if err != nil {
		// El UUID es válido, pero la persona no existe.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(
				codes.NotFound,
				"persona no encontrada",
			)
		}

		// Error inesperado de PostgreSQL.
		l.Errorf(
			"error consultando persona %s: %v",
			personaID,
			err,
		)

		return nil, status.Error(
			codes.Internal,
			"no se pudo consultar la persona",
		)
	}

	// Convertir el modelo almacenado en PostgreSQL
	// al mensaje protobuf utilizado por gRPC.
	persona := &maestros.Persona{
		Id:              p.ID,
		TipoDocumento:   p.TipoDocumento,
		NumeroDocumento: p.NumeroDocumento,
		Nombres:         p.Nombres,
		ApellidoPaterno: p.ApellidoPaterno,
		ApellidoMaterno: p.ApellidoMaterno,
		Activo:          p.Activo,

		CreatedAt: p.CreatedAt.Format(
			"2006-01-02T15:04:05Z07:00",
		),

		UpdatedAt: p.UpdatedAt.Format(
			"2006-01-02T15:04:05Z07:00",
		),
	}

	// Fecha de nacimiento es opcional.
	if p.FechaNacimiento != nil {
		persona.FechaNacimiento = p.FechaNacimiento.Format(
			"2006-01-02",
		)
	}

	// Sexo es opcional.
	if p.Sexo != nil {
		persona.Sexo = *p.Sexo
	}

	// Correo personal es opcional.
	if p.CorreoPersonal != nil {
		persona.CorreoPersonal = *p.CorreoPersonal
	}

	// Teléfono es opcional.
	if p.Telefono != nil {
		persona.Telefono = *p.Telefono
	}

	return &maestros.GetPersonaResponse{
		Persona: persona,
	}, nil
}
