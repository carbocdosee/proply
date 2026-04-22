package service

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrForbidden           = errors.New("forbidden")
	ErrConflict            = errors.New("conflict")
	ErrValidation          = errors.New("validation error")
	ErrEmailExists         = errors.New("email already exists")
	ErrPlanLimit           = errors.New("plan limit exceeded")
	ErrRevoked             = errors.New("proposal link has been revoked")
	ErrPlanRequired        = errors.New("plan upgrade required")
	ErrInvalidSignature    = errors.New("invalid webhook signature")
	ErrAlreadyProcessed    = errors.New("webhook already processed")
	ErrFileTooLarge        = errors.New("file too large")
	ErrInvalidContentType  = errors.New("invalid content type")
	ErrStorageNotConfigured = errors.New("object storage not configured")
)
