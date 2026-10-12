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

type UpdateEstudiantePersonalLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateEstudiantePersonalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateEstudiantePersonalLogic {
	return &UpdateEstudiantePersonalLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateEstudiantePersonal (FASE E-08): PATCH parcial con expected_version.
// Los punteros se pasan tal cual (nil = campo ausente). Responde el básico.
func (l *UpdateEstudiantePersonalLogic) UpdateEstudiantePersonal(req *types.UpdateEstudiantePersonalRequest) (resp *types.EstudianteWriteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.UpdateEstudiantePersonal(ctx, &estudiantesclient.UpdateEstudiantePersonalRequest{
		Id:               req.Id,
		ExpectedVersion:  req.ExpectedVersion,
		FechaNacimiento:  req.FechaNacimiento,
		Sexo:             req.Sexo,
		PaisNacimiento:   req.PaisNacimiento,
		UbigeoNacimiento: req.UbigeoNacimiento,
		LenguaMaterna:    req.LenguaMaterna,
		SegundaLengua:    req.SegundaLengua,
		// Modelo maestro: domicilio y contacto (punteros tal cual).
		DomicilioDireccion:    req.DomicilioDireccion,
		DomicilioDepartamento: req.DomicilioDepartamento,
		DomicilioProvincia:    req.DomicilioProvincia,
		DomicilioDistrito:     req.DomicilioDistrito,
		DomicilioUbigeo:       req.DomicilioUbigeo,
		Correo:                req.Correo,
		Telefono:              req.Telefono,
	})
	if err != nil {
		return nil, err
	}
	return &types.EstudianteWriteResponse{Estudiante: toEstudianteBasicoDTO(out.Estudiante)}, nil
}
