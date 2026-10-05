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

type GetPersonaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPersonaLogic(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
) *GetPersonaLogic {
	return &GetPersonaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPersonaLogic) GetPersona(
	req *types.GetPersonaRequest,
) (*types.GetPersonaResponse, error) {

	if req == nil || strings.TrimSpace(req.Id) == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"el id de la persona es obligatorio",
		)
	}

	personaID := strings.TrimSpace(req.Id)

	resp, err := l.svcCtx.MaestrosClient.GetPersona(
		l.ctx,
		&maestros.GetPersonaRequest{
			Id: personaID,
		},
	)

	if err != nil {
		l.Errorf(
			"error consultando persona %s en Maestros Service: %v",
			personaID,
			err,
		)

		return nil, err
	}

	if resp == nil || resp.Persona == nil {
		return nil, status.Error(
			codes.Internal,
			"Maestros Service devolvió una respuesta inválida",
		)
	}

	p := resp.Persona

	return &types.GetPersonaResponse{
		Persona: types.PersonaDTO{
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
		},
	}, nil
}
