package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/Yab1/golang-template/internal/platform/storage"
)

func (m *Module) SeedAdmin(ctx context.Context, email, username, password, role string) (*User, bool, error) {
	if email == "" || username == "" || password == "" {
		return nil, false, fmt.Errorf("SEED_ADMIN_EMAIL, SEED_ADMIN_USERNAME, and SEED_ADMIN_PASSWORD are required")
	}
	if role == "" {
		role = "admin"
	}

	existing, err := m.users.GetByEmail(ctx, email)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return nil, false, err
	}

	u := &User{
		Email:    email,
		Username: username,
		Role:     Role{Name: role},
	}
	if err := u.Password.Set(password); err != nil {
		return nil, false, err
	}
	if err := m.users.Create(ctx, u); err != nil {
		return nil, false, err
	}
	return u, true, nil
}
