package service

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrForbidden        = errors.New("forbidden")
	ErrConflict         = errors.New("conflict")
	ErrValidation       = errors.New("validation error")
	ErrEmailExists      = errors.New("email already exists")
	ErrPlanLimit        = errors.New("plan limit exceeded")
	ErrInvalidSignature = errors.New("invalid webhook signature")
	ErrAlreadyProcessed = errors.New("webhook already processed")
)
