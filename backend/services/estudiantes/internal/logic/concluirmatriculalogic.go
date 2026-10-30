package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConcluirMatriculaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConcluirMatriculaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConcluirMatriculaLogic {
	return &ConcluirMatriculaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ConcluirMatricula (etapa 5): el estudiante terminó el periodo (CONCLUIDO)
// — "estudiantes.matricula.update". Exige matrícula MATRICULADO y periodo
// EN_CURSO o CERRADO. No decide aprobación, promoción ni egreso: esas reglas
// no están definidas por el colegio.
func (l *ConcluirMatriculaLogic) ConcluirMatricula(in *estudiantes.ConcluirMatriculaRequest) (*estudiantes.MatriculaResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoMatriculaUpdate)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.ConcluirMatriculaRequest{}
	}
	id, err := validarID(in.MatriculaId)
	if err != nil {
		return nil, matriculaError(l.Logger, "concluir matrícula", err)
	}
	if err := validarVersion(in.ExpectedVersion); err != nil {
		return nil, matriculaError(l.Logger, "concluir matrícula", err)
	}
	fecha, err := validation.FechaMovimiento("fecha", in.Fecha, validation.HoyLima())
	if err != nil {
		return nil, matriculaError(l.Logger, "concluir matrícula", err)
	}
	out, err := l.svcCtx.EstudianteRepository.Concluir(l.ctx, id, int(in.ExpectedVersion), fecha, repository.Auditoria{UsuarioID: userID})
	if err != nil {
		return nil, matriculaError(l.Logger, "concluir matrícula", err)
	}
	return &estudiantes.MatriculaResponse{Matricula: matriculaToProto(out)}, nil
}
