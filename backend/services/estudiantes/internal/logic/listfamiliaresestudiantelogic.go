package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListFamiliaresEstudianteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFamiliaresEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFamiliaresEstudianteLogic {
	return &ListFamiliaresEstudianteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListFamiliaresEstudiante (modelo maestro, etapa 4): personas relacionadas
// con el estudiante, con parentesco, apoderado y vigencia. Requiere
// "estudiantes.familia.view"; cada lectura se audita (FAMILIA_VIEW) en la
// misma transacción, sin valores.
func (l *ListFamiliaresEstudianteLogic) ListFamiliaresEstudiante(in *estudiantes.ListFamiliaresEstudianteRequest) (*estudiantes.ListFamiliaresEstudianteResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoFamiliaView)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.ListFamiliaresEstudianteRequest{}
	}
	id, err := validarID(in.EstudianteId)
	if err != nil {
		return nil, escrituraError(l.Logger, "listar familiares", err)
	}
	familiares, err := l.svcCtx.EstudianteRepository.ListFamiliares(l.ctx, id, in.IncluirHistorial, repository.Auditoria{UsuarioID: userID})
	if err != nil {
		return nil, errorSeguro(l.Logger, "listar familiares", "no se pudieron consultar los familiares", err)
	}
	out := make([]*estudiantes.Familiar, 0, len(familiares))
	for i := range familiares {
		out = append(out, familiarToProto(&familiares[i]))
	}
	return &estudiantes.ListFamiliaresEstudianteResponse{Familiares: out}, nil
}
