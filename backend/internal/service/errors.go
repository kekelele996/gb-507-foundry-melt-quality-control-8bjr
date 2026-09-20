package service

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidTransition = errors.New("requested status transition is not allowed")
	ErrInvalidInput      = errors.New("business input validation failed")
	ErrConflict          = errors.New("concurrent or duplicate business action conflict")
	ErrUnauthorized      = errors.New("invalid username or password")
	ErrInactiveUser      = errors.New("user account is inactive")
)

func atomicValue[T any](ctx context.Context, security SecurityService, fn func(context.Context) (T, error)) (T, error) {
	var value T
	err := security.WithinTransaction(ctx, func(txCtx context.Context) error {
		var err error
		value, err = fn(txCtx)
		return err
	})
	return value, err
}

func atomicError(ctx context.Context, security SecurityService, fn func(context.Context) error) error {
	return security.WithinTransaction(ctx, fn)
}

func normalizeCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
