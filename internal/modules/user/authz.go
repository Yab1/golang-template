package user

import (
	"context"

	"github.com/google/uuid"

	"github.com/Yab1/golang-template/internal/platform/authz"
)

func (m *Module) ByID(ctx context.Context, id uuid.UUID) (*authz.Principal, error) {
	u, err := m.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &authz.Principal{
		ID:        u.ID,
		RoleName:  u.Role.Name,
		RoleLevel: u.Role.Level,
		TokenVer:  u.TokenVersion,
	}, nil
}

func (m *Module) LevelOf(ctx context.Context, role string) (int, error) {
	r, err := m.roles.GetByName(ctx, role)
	if err != nil {
		return 0, err
	}
	return r.Level, nil
}
