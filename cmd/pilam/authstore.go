package main

import (
	"context"
	"errors"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

var errStorePending = errors.New("auth: the user store is not wired yet")

type pendingUsers struct{}

func (pendingUsers) ByID(context.Context, ids.ID) (auth.User, error) {
	return auth.User{}, errStorePending
}

func (pendingUsers) ByEmail(context.Context, string) (auth.User, error) {
	return auth.User{}, errStorePending
}

func (pendingUsers) Create(context.Context, auth.User) error { return errStorePending }

func (pendingUsers) Update(context.Context, auth.User) error { return errStorePending }

type pendingAtomic struct{}

func (pendingAtomic) InTx(context.Context, func(context.Context) error) error {
	return errStorePending
}
