package logic

import (
	"context"
	"errors"
	"strings"

	"usuarios/internal/authorization"
	"usuarios/internal/interceptor"
	"usuarios/internal/repository"
	"usuarios/internal/svc"
	"usuarios/internal/validation"
	"usuarios/usuarios"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Verificación de identidad de solicitantes (Secretaría). Exige el permiso
// solicitantes.identidad.verify, comprobado contra Auth (no basta con el
// rol). El Gateway orquesta: Usuarios asigna primero el DNI definitivo; si
// Auth no puede verificar, se revierte. Los logs nunca llevan el DNI.

const permisoVerificarIdentidad = "solicitantes.identidad.verify"

func validarDNIVerificado(ctx context.Context, svcCtx *svc.ServiceContext, in *usuarios.VerifiedDniRequest) (string, string, error) {
	actorID, _, ok := interceptor.UserFromContext(ctx)
	if !ok {
		return "", "", status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if err := authorization.RequirePermission(ctx, svcCtx.AuthClient, actorID, permisoVerificarIdentidad); err != nil {
		return "", "", err
	}
	if in == nil {
		return "", "", status.Error(codes.InvalidArgument, "la solicitud es obligatoria")
	}
	id := strings.TrimSpace(in.Id)
	if !validation.IsValidUUID(id) {
		return "", "", status.Error(codes.InvalidArgument, "el identificador no es un UUID válido")
	}
	dni := strings.TrimSpace(in.Dni)
	if !dniPerfilPattern.MatchString(dni) {
		return "", "", status.Error(codes.InvalidArgument, "el DNI debe tener 8 dígitos numéricos")
	}
	if id == actorID {
		return "", "", status.Error(codes.PermissionDenied, "identidad_propia: No puedes verificar tu propia identidad.")
	}
	return id, dni, nil
}

type AssignVerifiedDniLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAssignVerifiedDniLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignVerifiedDniLogic {
	return &AssignVerifiedDniLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// AssignVerifiedDni: el DNI declarado pasa a ser el DNI definitivo del
// perfil SOLICITANTE. Si ya pertenece a otro perfil: conflicto, sin cambios.
func (l *AssignVerifiedDniLogic) AssignVerifiedDni(in *usuarios.VerifiedDniRequest) (*usuarios.VerifiedDniResponse, error) {
	id, dni, err := validarDNIVerificado(l.ctx, l.svcCtx, in)
	if err != nil {
		return nil, err
	}
	u, err := l.svcCtx.UsuarioRepository.AsignarDNIVerificado(l.ctx, id, dni)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrEsquemaPendiente):
			return nil, status.Error(codes.Unavailable, "identidad_no_disponible: La verificación de identidad todavía no está habilitada en Usuarios.")
		case errors.Is(err, repository.ErrUsuarioNotFound):
			return nil, status.Error(codes.NotFound, "identidad_sin_perfil: El solicitante no tiene perfil registrado.")
		case errors.Is(err, repository.ErrDniExists):
			return nil, status.Error(codes.AlreadyExists, "identidad_conflicto: El DNI declarado ya está asignado a otra cuenta. No se modificó ninguna cuenta.")
		case errors.Is(err, repository.ErrDNINoAsignable):
			return nil, status.Error(codes.FailedPrecondition, "identidad_no_asignable: El perfil no declaró ese DNI o ya tiene otro DNI.")
		}
		l.Errorf("asignar DNI verificado al perfil %s: %v", id, err)
		return nil, status.Error(codes.Internal, "no se pudo asignar el DNI verificado")
	}
	return &usuarios.VerifiedDniResponse{Usuario: toProto(u)}, nil
}

type RevertVerifiedDniLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRevertVerifiedDniLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevertVerifiedDniLogic {
	return &RevertVerifiedDniLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// RevertVerifiedDni: compensación si Auth no completó la verificación.
// Idempotente; nunca toca un perfil que no sea SOLICITANTE.
func (l *RevertVerifiedDniLogic) RevertVerifiedDni(in *usuarios.VerifiedDniRequest) (*usuarios.VerifiedDniResponse, error) {
	id, dni, err := validarDNIVerificado(l.ctx, l.svcCtx, in)
	if err != nil {
		return nil, err
	}
	u, err := l.svcCtx.UsuarioRepository.RevertirDNIVerificado(l.ctx, id, dni)
	if errors.Is(err, repository.ErrEsquemaPendiente) {
		return nil, status.Error(codes.Unavailable, "identidad_no_disponible: La verificación de identidad todavía no está habilitada en Usuarios.")
	}
	if err != nil {
		if errors.Is(err, repository.ErrUsuarioNotFound) {
			return nil, status.Error(codes.NotFound, "identidad_sin_perfil: El solicitante no tiene perfil registrado.")
		}
		l.Errorf("revertir DNI verificado del perfil %s: %v", id, err)
		return nil, status.Error(codes.Internal, "no se pudo revertir el DNI verificado")
	}
	return &usuarios.VerifiedDniResponse{Usuario: toProto(u)}, nil
}
