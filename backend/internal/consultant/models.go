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

// ServiceCommission is a per-service override of a consultant's default
// commission rate. Resolution order (session-level override → service-level
// override → consultant default) is implemented in M4's session module;
// this is just the config record.
type ServiceCommission struct {
	ID           string
	ConsultantID string
	ServiceID    string
	Commission   float64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

var (
	ErrNotFound          = errors.New("consultant not found")
	ErrUserNotFound      = errors.New("user not found")
	ErrAlreadyExists     = errors.New("consultant profile already exists for this user")
	ErrInvalidRole       = errors.New("user must have the clinician role")
	ErrValidation        = errors.New("full_name is required")
	ErrInvalidCommission = errors.New("default_commission must be between 0 and 100")
	ErrServiceNotFound   = errors.New("service not found")
)
