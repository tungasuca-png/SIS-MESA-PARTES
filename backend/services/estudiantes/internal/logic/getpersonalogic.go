package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPersonaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPersonaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPersonaLogic {
	return &GetPersonaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetPersona (etapa 5): la persona y los estudiantes con los que se relaciona
// (vigentes, o todos con historial) — "estudiantes.familia.view". Auditado
// como PERSONA_VIEW. De cada estudiante solo se devuelve la identidad básica.
func (l *GetPersonaLogic) GetPersona(in *estudiantes.GetPersonaRequest) (*estudiantes.GetPersonaResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoFamiliaView)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.GetPersonaRequest{}
	}
	id, err := validarID(in.Id)
	if err != nil {
		return nil, errorSeguro(l.Logger, "consultar persona", "no se pudo consultar la persona", err)
	}
	p, vinculos, err := l.svcCtx.EstudianteRepository.GetPersona(l.ctx, id, in.IncluirHistorial, repository.Auditoria{UsuarioID: userID})
	if err != nil {
		return nil, errorSeguro(l.Logger, "consultar persona", "no se pudo consultar la persona", err)
	}
	ids := make([]string, 0, len(vinculos))
	for _, v := range vinculos {
		ids = append(ids, v.EstudianteID)
	}
	basicos, err := l.svcCtx.EstudianteRepository.GetBasicByIDs(l.ctx, ids)
	if err != nil {
		return nil, errorSeguro(l.Logger, "consultar persona", "no se pudo consultar la persona", err)
	}
	porID := make(map[string]*estudiantes.EstudianteBasico, len(basicos))
	for _, e := range basicos {
		porID[e.ID] = toProtoBasic(e)
	}
	out := &estudiantes.GetPersonaResponse{Persona: personaToProto(p), Vinculos: make([]*estudiantes.VinculoPersona, 0, len(vinculos))}
	for _, v := range vinculos {
		out.Vinculos = append(out.Vinculos, &estudiantes.VinculoPersona{
			RelacionId: v.RelacionID, Estudiante: porID[v.EstudianteID], Parentesco: v.Parentesco,
			EsApoderado: v.EsApoderado, Vigente: v.Vigente, Desde: strOrEmpty(v.Desde), Hasta: strOrEmpty(v.Hasta),
		})
	}
	return out, nil
}
