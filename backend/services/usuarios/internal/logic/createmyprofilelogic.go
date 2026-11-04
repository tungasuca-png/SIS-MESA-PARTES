package logic

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"usuarios/internal/interceptor"
	"usuarios/internal/repository"
	"usuarios/internal/svc"
	"usuarios/internal/validation"
	"usuarios/usuarios"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const tipoSolicitante = "SOLICITANTE"

var dniPerfilPattern = regexp.MustCompile(`^[0-9]{8}$`)

type CreateMyProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateMyProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateMyProfileLogic {
	return &CreateMyProfileLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// CreateMyProfile: el SOLICITANTE registrado con Google crea SU perfil. El id
// es el "sub" del JWT (no hay id en la solicitud: sin IDOR) y el rol se toma
// del token firmado por Auth: solo un SOLICITANTE puede usarlo y el tipo
// queda fijo en SOLICITANTE. No sobrescribe un perfil existente.
func (l *CreateMyProfileLogic) CreateMyProfile(in *usuarios.CreateMyProfileRequest) (*usuarios.CreateMyProfileResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if role != tipoSolicitante {
		return nil, status.Error(codes.PermissionDenied, "solo un solicitante puede crear su propio perfil")
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "la solicitud es obligatoria")
	}
	nombres := strings.TrimSpace(in.Nombres)
	apellidos := strings.TrimSpace(in.Apellidos)
	if nombres == "" || apellidos == "" {
		return nil, status.Error(codes.InvalidArgument, "nombres y apellidos son obligatorios")
	}
	if utf8.RuneCountInString(nombres) > 100 || utf8.RuneCountInString(apellidos) > 100 {
		return nil, status.Error(codes.InvalidArgument, "nombres y apellidos no pueden superar los 100 caracteres")
	}
	// Registro público: el DNI solo se DECLARA (no se reserva el DNI
	// definitivo; lo asigna Secretaría al verificar la identidad).
	dni := strings.TrimSpace(in.Dni)
	dniDeclarado := strings.TrimSpace(in.DniDeclarado)
	if dniDeclarado != "" {
		if dni != "" || !dniPerfilPattern.MatchString(dniDeclarado) {
			return nil, status.Error(codes.InvalidArgument, "el DNI debe tener 8 dígitos numéricos")
		}
	} else if !dniPerfilPattern.MatchString(dni) {
		return nil, status.Error(codes.InvalidArgument, "el DNI debe tener 8 dígitos numéricos")
	}
	telefono := strings.TrimSpace(in.Telefono)
	if len(telefono) > maxTelefonoLength {
		return nil, status.Error(codes.InvalidArgument, "el teléfono no puede superar los 20 caracteres")
	}
	correo := strings.TrimSpace(in.Correo)
	if correo != "" && !validation.IsValidEmail(correo) {
		return nil, status.Error(codes.InvalidArgument, "el correo no tiene un formato válido")
	}
	direccion := strings.TrimSpace(in.Direccion)
	if utf8.RuneCountInString(direccion) > 255 {
		return nil, status.Error(codes.InvalidArgument, "la dirección no puede superar los 255 caracteres")
	}

	perfil, creado, err := l.svcCtx.UsuarioRepository.CreateIfNotExists(l.ctx, &repository.Usuario{
		ID: userID, Nombres: nombres, Apellidos: apellidos, DNI: dni, Telefono: telefono,
		Correo: correo, Direccion: direccion, TipoUsuario: tipoSolicitante, DNIDeclarado: dniDeclarado,
	})
	if err != nil {
		if errors.Is(err, repository.ErrEsquemaPendiente) {
			return nil, status.Error(codes.Unavailable, "el registro público no está disponible temporalmente")
		}
		if errors.Is(err, repository.ErrDniExists) {
			return nil, status.Error(codes.AlreadyExists, "el DNI ya está registrado para otro usuario")
		}
		l.Errorf("crear perfil propio %s: %v", userID, err)
		return nil, status.Error(codes.Internal, "no se pudo guardar el perfil")
	}
	return &usuarios.CreateMyProfileResponse{Usuario: toProto(perfil), Created: creado}, nil
}
