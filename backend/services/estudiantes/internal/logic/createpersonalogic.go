package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePersonaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePersonaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePersonaLogic {
	return &CreatePersonaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreatePersona (etapa 5): registra a un padre, madre, apoderado u otro
// familiar sin vincularlo todavía ("estudiantes.familia.update"). Con
// documento repetido responde duplicate_persona: nunca se duplica ni se
// fusiona. Auditado como PERSONA_UPDATE (version_resultante = 1).
func (l *CreatePersonaLogic) CreatePersona(in *estudiantes.CreatePersonaRequest) (*estudiantes.PersonaResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoFamiliaUpdate)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.CreatePersonaRequest{}
	}
	d, err := personaDatos(in.Persona)
	if err != nil {
		return nil, escrituraError(l.Logger, "registrar persona", err)
	}
	p, err := l.svcCtx.EstudianteRepository.CreatePersona(l.ctx, *d, repository.Auditoria{UsuarioID: userID})
	if err != nil {
		return nil, escrituraError(l.Logger, "registrar persona", err)
	}
	return &estudiantes.PersonaResponse{Persona: personaToProto(p)}, nil
}
