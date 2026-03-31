package service

import "errors"

// Sentinel errors returned by service methods.
// Handlers use errors.Is to map these to HTTP status codes.
var (
	ErrNotFound         = errors.New("not found")
	ErrAccessDenied     = errors.New("access denied")
	ErrConflict         = errors.New("conflict")
	ErrInvalidInput     = errors.New("invalid input")
	ErrEmailNotVerified = errors.New("email not verified")
	ErrWrongPassword    = errors.New("wrong password")
	ErrPasswordTooShort = errors.New("password too short")
)
