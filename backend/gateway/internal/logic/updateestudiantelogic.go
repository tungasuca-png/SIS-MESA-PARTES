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

type UpdateEstudianteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateEstudianteLogic {
	return &UpdateEstudianteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateEstudiante (FASE E-05): PATCH parcial con expected_version; el
// conflicto de versión llega como ABORTED (HTTP 409, "version_conflict").
func (l *UpdateEstudianteLogic) UpdateEstudiante(req *types.UpdateEstudianteRequest) (resp *types.EstudianteWriteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	// Los punteros se pasan tal cual: nil = campo ausente (no cambia).
	out, err := l.svcCtx.EstudiantesClient.UpdateEstudiante(ctx, &estudiantesclient.UpdateEstudianteRequest{
		Id:               req.Id,
		ExpectedVersion:  req.ExpectedVersion,
		CodigoSiagie:     req.CodigoSiagie,
		TipoDocumento:    req.TipoDocumento,
		NumeroDocumento:  req.NumeroDocumento,
		Nombres:          req.Nombres,
		ApellidoPaterno:  req.ApellidoPaterno,
		ApellidoMaterno:  req.ApellidoMaterno,
		Apellidos:        req.Apellidos,
		FechaNacimiento:  req.FechaNacimiento,
		Sexo:             req.Sexo,
		PaisNacimiento:   req.PaisNacimiento,
		UbigeoNacimiento: req.UbigeoNacimiento,
		LenguaMaterna:    req.LenguaMaterna,
		SegundaLengua:    req.SegundaLengua,
		FotoRef:          req.FotoRef,
	})
	if err != nil {
		return nil, err
	}
	return &types.EstudianteWriteResponse{Estudiante: toEstudianteBasicoDTO(out.Estudiante)}, nil
}
