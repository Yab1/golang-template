package main

import (
	"context"

	"github.com/google/uuid"

	"github.com/Yab1/golang-template/internal/modules/notification"
	"github.com/Yab1/golang-template/internal/modules/user"
)

type notificationAccounts struct {
	users *user.Module
}

func (a notificationAccounts) Account(ctx context.Context, id uuid.UUID) (notification.Account, error) {
	email, active, err := a.users.AccountContact(ctx, id)
	if err != nil {
		return notification.Account{}, err
	}
	return notification.Account{Email: email, Active: active}, nil
}
