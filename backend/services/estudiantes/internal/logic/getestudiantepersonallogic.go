package logic

import (
	"context"
	"strings"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"
	"estudiantes/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetEstudiantePersonalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetEstudiantePersonalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetEstudiantePersonalLogic {
	return &GetEstudiantePersonalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetEstudiantePersonal (FASE E-08): devuelve SOLO los seis datos personales
// (más id y version) con "estudiantes.personal.view". Cada lectura exitosa
// queda auditada como PERSONAL_VIEW (quién, cuándo, qué estudiante, versión
// observada y nombres de campos; nunca valores). No modifica la versión.
func (l *GetEstudiantePersonalLogic) GetEstudiantePersonal(in *estudiantes.GetEstudiantePersonalRequest) (*estudiantes.GetEstudiantePersonalResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoPersonalView)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.GetEstudiantePersonalRequest{}
	}
	id, err := validarID(in.Id)
	if err != nil {
		return nil, escrituraError(l.Logger, "leer datos personales", err)
	}

	// Fecha de referencia EXPLÍCITA para la edad (reportes a una fecha de
	// corte); vacía = hoy en Lima.
	referencia := validation.HoyLima()
	if f := strings.TrimSpace(in.FechaReferencia); f != "" {
		if referencia, err = validation.Fecha("fecha_referencia", f); err != nil {
			return nil, escrituraError(l.Logger, "leer datos personales", err)
		}
	}

	p, err := l.svcCtx.EstudianteRepository.GetPersonal(l.ctx, id, repository.Auditoria{UsuarioID: userID})
	if err != nil {
		return nil, errorSeguro(l.Logger, "leer datos personales", "no se pudieron consultar los datos personales", err)
	}
	return &estudiantes.GetEstudiantePersonalResponse{Personal: personalToProto(id, p, referencia)}, nil
}
