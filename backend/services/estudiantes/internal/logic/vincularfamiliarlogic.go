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

type VincularFamiliarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVincularFamiliarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VincularFamiliarLogic {
	return &VincularFamiliarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// VincularFamiliar (modelo maestro, etapa 4): relaciona una persona con el
// estudiante (padre, madre, tutor, etc.) con "estudiantes.familia.update".
// La persona se identifica por persona_id o por sus datos; con documento se
// reutiliza la existente si los nombres coinciden (nunca se sobrescribe ni
// se fusiona). Reglas de la base: una relación vigente por persona, un solo
// PADRE y una sola MADRE vigentes, un solo apoderado vigente. Auditado como
// FAMILIA_UPDATE (sin valores).
func (l *VincularFamiliarLogic) VincularFamiliar(in *estudiantes.VincularFamiliarRequest) (*estudiantes.VincularFamiliarResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, permisoFamiliaUpdate)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.VincularFamiliarRequest{}
	}
	estudianteID, err := validarID(in.EstudianteId)
	if err != nil {
		return nil, escrituraError(l.Logger, "vincular familiar", err)
	}
	vinculo := repository.VinculoInput{
		Parentesco:  validation.Upper(in.Parentesco),
		EsApoderado: in.EsApoderado,
	}
	if err := validation.Parentesco(vinculo.Parentesco); err != nil {
		return nil, escrituraError(l.Logger, "vincular familiar", err)
	}
	if vinculo.Desde, err = fechaOpcional("desde", in.Desde); err != nil {
		return nil, escrituraError(l.Logger, "vincular familiar", err)
	}

	personaID := strings.TrimSpace(in.PersonaId)
	switch {
	case personaID != "" && in.Persona != nil:
		return nil, escrituraError(l.Logger, "vincular familiar", validation.Invalid("indique persona_id o los datos de la persona, no ambos"))
	case personaID != "":
		if vinculo.PersonaID, err = validarID(personaID); err != nil {
			return nil, escrituraError(l.Logger, "vincular familiar", err)
		}
	default:
		if vinculo.Persona, err = personaDatos(in.Persona); err != nil {
			return nil, escrituraError(l.Logger, "vincular familiar", err)
		}
	}

	f, err := l.svcCtx.EstudianteRepository.VincularFamiliar(l.ctx, estudianteID, vinculo, repository.Auditoria{UsuarioID: userID})
	if err != nil {
		return nil, escrituraError(l.Logger, "vincular familiar", err)
	}
	return &estudiantes.VincularFamiliarResponse{Familiar: familiarToProto(f)}, nil
}
