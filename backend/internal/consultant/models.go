package consultant

import (
	"errors"
	"time"
)

type Consultant struct {
	ID                string
	UserID            string
	FullName          string
	DefaultCommission float64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

var (
	ErrNotFound          = errors.New("consultant not found")
	ErrUserNotFound      = errors.New("user not found")
	ErrAlreadyExists     = errors.New("consultant profile already exists for this user")
	ErrInvalidRole       = errors.New("user must have the clinician role")
	ErrValidation        = errors.New("full_name is required")
	ErrInvalidCommission = errors.New("default_commission must be between 0 and 100")
)
