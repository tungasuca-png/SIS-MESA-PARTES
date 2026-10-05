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

type GetEstudianteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetEstudianteLogic(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
) *GetEstudianteLogic {
	return &GetEstudianteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetEstudianteLogic) GetEstudiante(
	in *maestros.GetEstudianteRequest,
) (*maestros.GetEstudianteResponse, error) {

	// Validar que el ID haya sido enviado.
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"el id del estudiante es obligatorio",
		)
	}

	estudianteID := strings.TrimSpace(in.Id)

	// Validar formato UUID.
	if _, err := uuid.Parse(estudianteID); err != nil {
		return nil, status.Error(
			codes.InvalidArgument,
			"el id del estudiante debe ser un UUID válido",
		)
	}

	// Buscar estudiante en maestros_db.
	estudiante, err := l.svcCtx.EstudianteRepository.FindByID(
		l.ctx,
		estudianteID,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(
				codes.NotFound,
				"estudiante no encontrado",
			)
		}

		l.Errorf(
			"error consultando estudiante %s: %v",
			estudianteID,
			err,
		)

		return nil, status.Error(
			codes.Internal,
			"no se pudo consultar el estudiante",
		)
	}

	// Buscar la Persona asociada al estudiante.
	persona, err := l.svcCtx.PersonaRepository.FindByID(
		l.ctx,
		estudiante.PersonaID,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			l.Errorf(
				"persona %s asociada al estudiante %s no encontrada",
				estudiante.PersonaID,
				estudianteID,
			)

			return nil, status.Error(
				codes.Internal,
				"el estudiante no tiene una persona institucional válida",
			)
		}

		l.Errorf(
			"error consultando persona %s del estudiante %s: %v",
			estudiante.PersonaID,
			estudianteID,
			err,
		)

		return nil, status.Error(
			codes.Internal,
			"no se pudo consultar la información personal del estudiante",
		)
	}

	// Convertir Persona de PostgreSQL al mensaje protobuf.
	personaResponse := &maestros.Persona{
		Id:              persona.ID,
		TipoDocumento:   persona.TipoDocumento,
		NumeroDocumento: persona.NumeroDocumento,
		Nombres:         persona.Nombres,
		ApellidoPaterno: persona.ApellidoPaterno,
		ApellidoMaterno: persona.ApellidoMaterno,
		Activo:          persona.Activo,
		CreatedAt: persona.CreatedAt.Format(
			"2006-01-02T15:04:05Z07:00",
		),
		UpdatedAt: persona.UpdatedAt.Format(
			"2006-01-02T15:04:05Z07:00",
		),
	}

	// Campos opcionales de Persona.
	if persona.FechaNacimiento != nil {
		personaResponse.FechaNacimiento = persona.FechaNacimiento.Format(
			"2006-01-02",
		)
	}

	if persona.Sexo != nil {
		personaResponse.Sexo = *persona.Sexo
	}

	if persona.CorreoPersonal != nil {
		personaResponse.CorreoPersonal = *persona.CorreoPersonal
	}

	if persona.Telefono != nil {
		personaResponse.Telefono = *persona.Telefono
	}

	// Construir respuesta del estudiante.
	estudianteResponse := &maestros.Estudiante{
		Id:                  estudiante.ID,
		PersonaId:           estudiante.PersonaID,
		CodigoInstitucional: estudiante.CodigoInstitucional,
		EstadoAcademico:     estudiante.EstadoAcademico,
		CreatedAt: estudiante.CreatedAt.Format(
			"2006-01-02T15:04:05Z07:00",
		),
		UpdatedAt: estudiante.UpdatedAt.Format(
			"2006-01-02T15:04:05Z07:00",
		),
		Persona: personaResponse,
	}

	// Código SIAGIE puede ser NULL.
	if estudiante.CodigoSiagie != nil {
		estudianteResponse.CodigoSiagie = *estudiante.CodigoSiagie
	}

	return &maestros.GetEstudianteResponse{
		Estudiante: estudianteResponse,
	}, nil
}
