package apperror

import "errors"

var (
	ErrBadRequest    = errors.New("bad request")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrInternal      = errors.New("internal error")
	ErrInvalidSecret = errors.New("invalid secret")
)
