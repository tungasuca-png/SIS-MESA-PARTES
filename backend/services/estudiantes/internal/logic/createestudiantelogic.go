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

type CreateEstudianteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateEstudianteLogic {
	return &CreateEstudianteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// tienePersonalesAlta: algún dato personal con contenido (solo espacios
// cuenta como vacío: no se escribiría nada).
func tienePersonalesAlta(in *estudiantes.CreateEstudianteRequest) bool {
	for _, v := range []string{in.FechaNacimiento, in.Sexo, in.PaisNacimiento, in.UbigeoNacimiento, in.LenguaMaterna, in.SegundaLengua} {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// CreateEstudiante (FASE E-05) registra SOLO el maestro básico, con
// "estudiantes.create". El código institucional lo envía el cliente (no hay
// generador: el modelo no define uno) y debe ser único. Nivel/grado/sección/
// turno y estado_matricula quedan con sus valores por defecto: se asignarán
// en el dominio Matrícula. Alta + auditoría en una sola transacción.
// FASE E-08: con datos personales exige además estudiantes.personal.update.
func (l *CreateEstudianteLogic) CreateEstudiante(in *estudiantes.CreateEstudianteRequest) (*estudiantes.CreateEstudianteResponse, error) {
	userID, err := autorizar(l.ctx, l.svcCtx, "estudiantes.create")
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.CreateEstudianteRequest{}
	}
	// FASE E-08: los datos personales en el alta exigen además
	// "estudiantes.personal.update". Se comprueba ANTES de validar o escribir:
	// sin ese permiso no se crea nada.
	if tienePersonalesAlta(in) {
		if err := requierePermisoPersonal(l.ctx, l.svcCtx, userID); err != nil {
			return nil, err
		}
	}

	m := repository.Maestro{
		Codigo:           validation.CleanText(in.Codigo),
		CodigoSiagie:     optTrim(in.CodigoSiagie),
		TipoDocumento:    validation.Upper(in.TipoDocumento),
		NumeroDocumento:  optUpper(in.NumeroDocumento),
		Nombres:          validation.CleanText(in.Nombres),
		ApellidoPaterno:  optClean(in.ApellidoPaterno),
		ApellidoMaterno:  optClean(in.ApellidoMaterno),
		FechaNacimiento:  optTrim(in.FechaNacimiento),
		Sexo:             optUpper(in.Sexo),
		PaisNacimiento:   optUpper(in.PaisNacimiento),
		UbigeoNacimiento: optTrim(in.UbigeoNacimiento),
		LenguaMaterna:    optUpper(in.LenguaMaterna),
		SegundaLengua:    optUpper(in.SegundaLengua),
		FotoRef:          optTrim(in.FotoRef),
		Estado:           validation.Upper(in.Estado),
	}
	if m.TipoDocumento == "" {
		m.TipoDocumento = validation.TipoSinDocumento
	}
	if m.Estado == "" {
		m.Estado = validation.EstadoRegistroActivo
	}
	apellidos := in.Apellidos
	if err := resolverApellidos(&m, &apellidos); err != nil {
		return nil, escrituraError(l.Logger, "crear", err)
	}
	if err := validarMaestro(&m); err != nil {
		return nil, escrituraError(l.Logger, "crear", err)
	}

	// Campos informados en el alta (solo nombres, para la auditoría).
	campos := camposModificados(&repository.Maestro{}, &m)
	created, err := l.svcCtx.EstudianteRepository.Create(l.ctx, m, repository.Auditoria{
		UsuarioID: userID, Accion: repository.AccionCreate, Campos: campos,
	})
	if err != nil {
		return nil, escrituraError(l.Logger, "crear", err)
	}
	return &estudiantes.CreateEstudianteResponse{Estudiante: respuestaBasica(created)}, nil
}
