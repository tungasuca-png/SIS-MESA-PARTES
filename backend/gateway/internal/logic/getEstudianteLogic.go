package logic

import (
	"context"
	"strings"

	"gateway/internal/svc"
	"gateway/internal/types"
	"maestros/maestros"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetEstudianteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetEstudianteLogic(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
) *GetEstudianteLogic {
	return &GetEstudianteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetEstudianteLogic) GetEstudiante(
	req *types.GetEstudianteRequest,
) (*types.GetEstudianteResponse, error) {

	if req == nil || strings.TrimSpace(req.Id) == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"el id del estudiante es obligatorio",
		)
	}

	estudianteID := strings.TrimSpace(req.Id)

	resp, err := l.svcCtx.MaestrosClient.GetEstudiante(
		l.ctx,
		&maestros.GetEstudianteRequest{
			Id: estudianteID,
		},
	)

	if err != nil {
		l.Errorf(
			"error consultando estudiante %s en Maestros Service: %v",
			estudianteID,
			err,
		)

		return nil, err
	}

	if resp == nil || resp.Estudiante == nil {
		return nil, status.Error(
			codes.Internal,
			"Maestros Service devolvió una respuesta inválida",
		)
	}

	e := resp.Estudiante

	estudiante := types.EstudianteDTO{
		Id:                  e.Id,
		PersonaId:           e.PersonaId,
		CodigoInstitucional: e.CodigoInstitucional,
		CodigoSiagie:        e.CodigoSiagie,
		EstadoAcademico:     e.EstadoAcademico,
		CreatedAt:           e.CreatedAt,
		UpdatedAt:           e.UpdatedAt,
	}

	if e.Persona != nil {
		p := e.Persona

		estudiante.Persona = types.PersonaDTO{
			Id:              p.Id,
			TipoDocumento:   p.TipoDocumento,
			NumeroDocumento: p.NumeroDocumento,
			Nombres:         p.Nombres,
			ApellidoPaterno: p.ApellidoPaterno,
			ApellidoMaterno: p.ApellidoMaterno,
			FechaNacimiento: p.FechaNacimiento,
			Sexo:            p.Sexo,
			CorreoPersonal:  p.CorreoPersonal,
			Telefono:        p.Telefono,
			Activo:          p.Activo,
			CreatedAt:       p.CreatedAt,
			UpdatedAt:       p.UpdatedAt,
		}
	}

	return &types.GetEstudianteResponse{
		Estudiante: estudiante,
	}, nil
}
