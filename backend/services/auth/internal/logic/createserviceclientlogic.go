package logic

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"auth/auth"
	"auth/internal/interceptor"
	"auth/internal/repository"
	"auth/internal/security"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateServiceClientLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateServiceClientLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateServiceClientLogic {
	return &CreateServiceClientLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateServiceClientLogic) CreateServiceClient(in *auth.CreateServiceClientRequest) (*auth.CreateServiceClientResponse, error) {
	adminID, ok := interceptor.UserIDFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	nombre := strings.TrimSpace(in.GetNombre())
	if nombre == "" || utf8.RuneCountInString(nombre) > 120 {
		return nil, status.Error(codes.InvalidArgument, "el nombre del sistema es obligatorio (máximo 120 caracteres)")
	}
	permisos, err := validarPermisosSistema(in.GetPermisos())
	if err != nil {
		return nil, err
	}
	clientID, secret, hash, err := security.GenerateClientCredentials()
	if err != nil {
		return nil, status.Error(codes.Internal, "no se pudieron generar las credenciales")
	}
	created, err := l.svcCtx.ServiceClientRepository.Create(l.ctx, nombre, clientID, hash, adminID, permisos)
	if err != nil {
		if errors.Is(err, repository.ErrPermisoInexistente) {
			return nil, status.Error(codes.InvalidArgument, "algún permiso no existe o está inactivo en el catálogo")
		}
		l.Errorf("registrar sistema consumidor: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo registrar el sistema")
	}
	l.Infof("sistema consumidor registrado: id=%s client_id=%s por=%s permisos=%q", created.ID, created.ClientID, adminID, strings.Join(created.Permisos, " "))
	return &auth.CreateServiceClientResponse{Client: serviceClientProto(created), ClientSecret: secret}, nil
}
