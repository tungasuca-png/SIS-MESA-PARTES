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

type VincularFamiliarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewVincularFamiliarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VincularFamiliarLogic {
	return &VincularFamiliarLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// VincularFamiliar (modelo maestro): permiso (estudiantes.familia.update),
// validación y auditoría en Estudiantes Service.
func (l *VincularFamiliarLogic) VincularFamiliar(req *types.VincularFamiliarRequest) (resp *types.FamiliarResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	in := &estudiantesclient.VincularFamiliarRequest{
		EstudianteId: req.Id,
		PersonaId:    req.PersonaId,
		Parentesco:   req.Parentesco,
		EsApoderado:  req.EsApoderado,
		Desde:        req.Desde,
	}
	if p := req.Persona; p != nil {
		in.Persona = &estudiantesclient.Persona{
			TipoDocumento: p.TipoDocumento, NumeroDocumento: p.NumeroDocumento, Nombres: p.Nombres,
			ApellidoPaterno: p.ApellidoPaterno, ApellidoMaterno: p.ApellidoMaterno,
		}
	}
	out, err := l.svcCtx.EstudiantesClient.VincularFamiliar(ctx, in)
	if err != nil {
		return nil, err
	}
	return &types.FamiliarResponse{Familiar: toFamiliarDTO(out.Familiar)}, nil
}
