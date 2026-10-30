package logic

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"auth/auth"
	"auth/internal/repository"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateUserLogic {
	return &CreateUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// createUserUUIDPattern: mismo patrón ya usado por HasPermissionLogic/
// UpdateUserRoleLogic/UpdateRolePermissionsLogic.
var createUserUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// CreateUser (administrativo, distinto de Register): autenticación +
// "usuarios.change_role" ya resueltas por el interceptor (ProtectedMethod).
// A diferencia de Register (público, self-service, solo SOLICITANTE), acá
// el actor puede asignar cualquier rol real ACTIVO mediante role_id — nunca
// un nombre de rol, y siempre validado contra la BD, nunca confiando en el
// frontend. Reutiliza las mismas reglas de username/email/password que ya
// usa Register (RegisterLogic.Register), sin inventar restricciones nuevas.
func (l *CreateUserLogic) CreateUser(in *auth.CreateUserRequest) (*auth.CreateUserResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "la solicitud es obligatoria")
	}

	username := strings.TrimSpace(in.Username)
	email := strings.TrimSpace(strings.ToLower(in.Email))
	password := in.Password
	roleID := strings.TrimSpace(in.RoleId)

	dni, err := normalizeDNI(in.Dni)
	if err != nil {
		return nil, err
	}

	if username == "" {
		return nil, status.Error(codes.InvalidArgument, "el username es obligatorio")
	}
	if utf8.RuneCountInString(username) < 3 {
		return nil, status.Error(codes.InvalidArgument, "el username debe tener al menos 3 caracteres")
	}

	if email == "" {
		return nil, status.Error(codes.InvalidArgument, "el email es obligatorio")
	}
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || parsedEmail.Address != email {
		return nil, status.Error(codes.InvalidArgument, "el email no tiene un formato válido")
	}

	if password == "" {
		return nil, status.Error(codes.InvalidArgument, "la contraseña es obligatoria")
	}
	if err := validarPassword(password); err != nil {
		return nil, err
	}

	// role_id es obligatorio: "1 usuario = 1 rol" no admite crear un usuario
	// sin ningún rol (a diferencia de Register, que siempre tiene un rol
	// implícito -- SOLICITANTE -- esta operación administrativa exige que
	// el actor elija uno explícitamente).
	if roleID == "" {
		return nil, status.Error(codes.InvalidArgument, "el role_id es obligatorio")
	}
	if !createUserUUIDPattern.MatchString(roleID) {
		return nil, status.Error(codes.InvalidArgument, "el role_id no tiene un formato válido")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudo procesar la contraseña")
	}

	created, role, err := l.svcCtx.UserRepository.CreateUserWithRoleID(
		l.ctx,
		&repository.User{
			Username:     username,
			DNI:          dni,
			Email:        email,
			PasswordHash: string(passwordHash),
		},
		roleID,
	)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrUsernameExists):
			return nil, status.Error(codes.AlreadyExists, "el username ya existe")
		case errors.Is(err, repository.ErrDNIExists):
			return nil, status.Error(codes.AlreadyExists, "el DNI ya existe")
		case errors.Is(err, repository.ErrEmailExists):
			return nil, status.Error(codes.AlreadyExists, "el email ya existe")
		case errors.Is(err, repository.ErrRoleNotFound):
			return nil, status.Error(codes.NotFound, "rol no encontrado")
		case errors.Is(err, repository.ErrRoleInactive):
			return nil, status.Error(codes.FailedPrecondition, "el rol está inactivo")
		case errors.Is(err, repository.ErrRoleAssignment):
			return nil, status.Error(codes.Internal, "no se pudo asignar el rol")
		default:
			l.Errorf("crear usuario %s: %v", username, err)
			return nil, status.Error(codes.Internal, "no se pudo registrar el usuario")
		}
	}

	return &auth.CreateUserResponse{
		Id:         created.ID,
		Username:   created.Username,
		Email:      created.Email,
		RoleId:     role.ID,
		RoleNombre: role.Name,
	}, nil
}
