package logic

import (
	"context"
	"errors"
	"net/mail"
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

type RegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logger logx.Logger
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		logger: logx.WithContext(ctx),
	}
}

func (l *RegisterLogic) Register(in *auth.RegisterRequest) (*auth.RegisterResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "la solicitud es obligatoria")
	}

	username := strings.TrimSpace(in.Username)
	email := strings.TrimSpace(strings.ToLower(in.Email))
	password := in.Password
	role := strings.TrimSpace(strings.ToUpper(in.Role))

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
	if len(password) < 8 {
		return nil, status.Error(codes.InvalidArgument, "la contraseña debe tener al menos 8 caracteres")
	}

	if role == "" {
		role = "SOLICITANTE"
	}

	switch role {
	case "SOLICITANTE":
	case "ADMIN", "DIRECTOR", "SUBDIRECTOR", "SECRETARIA", "DOCENTE", "AUXILIAR":
		return nil, status.Error(codes.PermissionDenied, "no se permite registrar ese rol públicamente")
	default:
		return nil, status.Error(codes.InvalidArgument, "el rol indicado no existe")
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return nil, status.Error(
			codes.Internal,
			"no se pudo procesar la contraseña",
		)
	}

	created, err := l.svcCtx.UserRepository.CreateUserWithRole(
		l.ctx,
		&repository.User{
			Username:     username,
			Email:        email,
			PasswordHash: string(passwordHash),
		},
		role,
	)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrUsernameExists):
			return nil, status.Error(codes.AlreadyExists, "el username ya existe")
		case errors.Is(err, repository.ErrEmailExists):
			return nil, status.Error(codes.AlreadyExists, "el email ya existe")
		case errors.Is(err, repository.ErrRoleNotFound):
			return nil, status.Error(codes.InvalidArgument, "el rol indicado no existe")
		case errors.Is(err, repository.ErrRoleAssignment):
			return nil, status.Error(codes.Internal, "no se pudo asignar el rol")
		default:
			return nil, status.Error(codes.Internal, "no se pudo registrar el usuario")
		}
	}

	return &auth.RegisterResponse{
		Id:       created.ID,
		Username: created.Username,
		Email:    created.Email,
		Role:     role,
		Message:  "usuario registrado correctamente",
	}, nil
}
