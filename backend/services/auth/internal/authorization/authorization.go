package authorization

import (
	"context"
	"errors"
	"sort"
	"strings"

	"auth/internal/repository"
)

type userLookup interface {
	FindByID(ctx context.Context, userID string) (*repository.User, error)
	GetRoles(ctx context.Context, userID string) ([]string, error)
}

type roleLookup interface {
	FindByName(ctx context.Context, name string) (*repository.Role, error)
}

type permissionLookup interface {
	GetPermissionsByRoleID(ctx context.Context, roleID string) ([]*repository.Permission, error)
}

type AuthorizationService struct {
	userRepository       userLookup
	roleRepository       roleLookup
	rolePermissionRepo   permissionLookup
}

func NewAuthorizationService(
	userRepository userLookup,
	roleRepository roleLookup,
	rolePermissionRepo permissionLookup,
) *AuthorizationService {
	return &AuthorizationService{
		userRepository:     userRepository,
		roleRepository:     roleRepository,
		rolePermissionRepo: rolePermissionRepo,
	}
}

func (s *AuthorizationService) GetPermissionsByUserID(ctx context.Context, userID string) ([]string, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, nil
	}

	user, err := s.userRepository.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if !user.Estado {
		return nil, nil
	}

	roleNames, err := s.userRepository.GetRoles(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(roleNames) == 0 {
		return nil, nil
	}

	permissionSet := make(map[string]struct{})
	for _, roleName := range roleNames {
		role, err := s.roleRepository.FindByName(ctx, roleName)
		if err != nil {
			if errors.Is(err, repository.ErrRoleNotFound) {
				continue
			}
			return nil, err
		}

		permissions, err := s.rolePermissionRepo.GetPermissionsByRoleID(ctx, role.ID)
		if err != nil {
			return nil, err
		}
		for _, permission := range permissions {
			if permission == nil || !permission.Estado {
				continue
			}
			permissionSet[permission.Codigo] = struct{}{}
		}
	}

	codes := make([]string, 0, len(permissionSet))
	for code := range permissionSet {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes, nil
}

func (s *AuthorizationService) HasPermission(ctx context.Context, userID string, permissionCode string) (bool, error) {
	permissionCode = strings.TrimSpace(permissionCode)
	if strings.TrimSpace(userID) == "" || permissionCode == "" {
		return false, nil
	}

	permissions, err := s.GetPermissionsByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, permission := range permissions {
		if permission == permissionCode {
			return true, nil
		}
	}

	return false, nil
}
