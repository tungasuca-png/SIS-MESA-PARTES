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

type GetPersonaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPersonaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPersonaLogic {
	return &GetPersonaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetPersona (etapa 5): persona y sus vínculos con estudiantes (estudiantes.familia.view, auditado). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *GetPersonaLogic) GetPersona(req *types.GetPersonaRequest) (resp *types.GetPersonaResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.GetPersona(ctx, &estudiantesclient.GetPersonaRequest{Id: req.PersonaId, IncluirHistorial: req.Historial})
	if err != nil {
		return nil, err
	}
	vinculos := make([]types.VinculoPersonaDTO, 0, len(out.Vinculos))
	for _, v := range out.Vinculos {
		vinculos = append(vinculos, types.VinculoPersonaDTO{
			RelacionId: v.GetRelacionId(), Estudiante: toEstudianteBasicoPtr(v.GetEstudiante()), Parentesco: v.GetParentesco(),
			EsApoderado: v.GetEsApoderado(), Vigente: v.GetVigente(), Desde: v.GetDesde(), Hasta: v.GetHasta(),
		})
	}
	return &types.GetPersonaResponse{Persona: toPersonaDTO(out.Persona), Vinculos: vinculos}, nil
}
