// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"estudiantes/estudiantesclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePersonaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdatePersonaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePersonaLogic {
	return &UpdatePersonaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdatePersona (etapa 5): PATCH parcial con versión (estudiantes.familia.update). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *UpdatePersonaLogic) UpdatePersona(req *types.UpdatePersonaRequest) (resp *types.PersonaResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.UpdatePersona(ctx, &estudiantesclient.UpdatePersonaRequest{
		Id: req.PersonaId, ExpectedVersion: req.ExpectedVersion, TipoDocumento: req.TipoDocumento, NumeroDocumento: req.NumeroDocumento,
		Nombres: req.Nombres, ApellidoPaterno: req.ApellidoPaterno, ApellidoMaterno: req.ApellidoMaterno,
	})
	if err != nil {
		return nil, err
	}
	return &types.PersonaResponse{Persona: toPersonaDTO(out.Persona)}, nil
}
