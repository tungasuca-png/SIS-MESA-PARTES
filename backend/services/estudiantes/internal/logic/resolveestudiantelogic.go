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

type ResolveEstudianteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveEstudianteLogic {
	return &ResolveEstudianteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResolveEstudiante (modelo maestro; etapa 5 agrega código SIAGIE): resolución EXACTA para sistemas
// consumidores, por código institucional o por tipo + número de documento
// (exactamente uno de los dos criterios). Requiere "estudiantes.view" y
// devuelve solo la identidad básica. No hay coincidencias parciales: para
// eso está SearchEstudiantes.
func (l *ResolveEstudianteLogic) ResolveEstudiante(in *estudiantes.ResolveEstudianteRequest) (*estudiantes.ResolveEstudianteResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, "estudiantes.view"); err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.ResolveEstudianteRequest{}
	}
	codigo := validation.CleanText(in.Codigo)
	siagie := strings.TrimSpace(in.CodigoSiagie)
	tipo := validation.Upper(in.TipoDocumento)
	numero := validation.Upper(in.NumeroDocumento)
	porDocumento := tipo != "" || numero != ""

	criterios := 0
	for _, usado := range []bool{codigo != "", siagie != "", porDocumento} {
		if usado {
			criterios++
		}
	}

	var (
		found *repository.Estudiante
		err   error
	)
	switch {
	case criterios > 1:
		return nil, escrituraError(l.Logger, "resolver", validation.Invalid("use un solo criterio: codigo, codigo_siagie o tipo_documento + numero_documento"))
	case siagie != "":
		if err := validation.CodigoSiagie(&siagie); err != nil {
			return nil, escrituraError(l.Logger, "resolver", err)
		}
		found, err = l.svcCtx.EstudianteRepository.GetByCodigoSiagie(l.ctx, siagie)
	case codigo != "":
		if err := validation.RequiredText("codigo", codigo, validation.MaxCodigo); err != nil {
			return nil, escrituraError(l.Logger, "resolver", err)
		}
		found, err = l.svcCtx.EstudianteRepository.GetByCodigo(l.ctx, codigo)
	case porDocumento:
		if tipo == validation.TipoSinDocumento {
			return nil, escrituraError(l.Logger, "resolver", validation.Invalid("SIN_DOCUMENTO no permite resolver por documento"))
		}
		if err := validation.Documento(tipo, &numero); err != nil {
			return nil, escrituraError(l.Logger, "resolver", err)
		}
		found, err = l.svcCtx.EstudianteRepository.GetByDocumento(l.ctx, tipo, strings.TrimSpace(numero))
	default:
		return nil, escrituraError(l.Logger, "resolver", validation.Invalid("indique codigo, codigo_siagie o tipo_documento + numero_documento"))
	}
	if err != nil {
		return nil, errorSeguro(l.Logger, "resolver", "no se pudo resolver el estudiante", err)
	}
	return &estudiantes.ResolveEstudianteResponse{Estudiante: toProtoBasic(found)}, nil
}
