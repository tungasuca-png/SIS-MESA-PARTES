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

type CreatePersonaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePersonaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePersonaLogic {
	return &CreatePersonaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CreatePersona (etapa 5): registra una persona sin vincularla (estudiantes.familia.update). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *CreatePersonaLogic) CreatePersona(req *types.CreatePersonaRequest) (resp *types.PersonaResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.CreatePersona(ctx, &estudiantesclient.CreatePersonaRequest{Persona: toPersonaInput(req.Persona)})
	if err != nil {
		return nil, err
	}
	return &types.PersonaResponse{Persona: toPersonaDTO(out.Persona)}, nil
}
